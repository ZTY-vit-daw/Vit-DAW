// Package vps implements the VPS v3 Foundation described by ADR-SPAL-001.
//
// VPS is the user-level source of truth for plugin learning, control-surface
// knowledge, Provider Credentials and their conformance evidence.  A VPS is
// deliberately not a project plugin instance: project-specific loading,
// resource leases and receipts belong to later SPAL execution phases.
package vps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
)

const (
	// DocumentSchemaVersion is the schema of one canonical VPS document.
	DocumentSchemaVersion = "vit.vps.v3"
	// LibrarySchemaVersion is the schema of the user-level VPS Library file.
	LibrarySchemaVersion = "vit.vps_library.v3"
	// CatalogSchemaVersion is the schema of the derived, non-authoritative
	// Provider Catalog read model.
	CatalogSchemaVersion = "vit.provider_catalog.v3"

	// StaticEQCapabilityID is the first VPS v3 capability profile. It is
	// intentionally distinct from the SPAL v0 spectral.static_bell.v1 API.
	StaticEQCapabilityID   = "spectral.static_eq.v0"
	StaticEQProfileVersion = "v0"
	// EqualizerCapabilityID is the SPAL EQ v2 work-card Credential.  One
	// Credential may expose several conformed schemas, but dispatch remains
	// limited to its persisted capability matrix.
	EqualizerCapabilityID   = "equalizer.v2"
	EqualizerProfileVersion = "v2"
	// FXMMeasurementRequirementSchema is an authoring-time contract. It does
	// not grant dispatch authority; it records which counterfactual effect
	// measurements a reusable VPS still needs or has evidenced.
	FXMMeasurementRequirementSchema = "vit.vps.fxm_measurement_requirement.v0"

	OperationAllocateBand       = "allocate_band"
	OperationPatchBand          = "patch_band"
	OperationReadBand           = "read_band"
	OperationReleaseBand        = "release_band"
	OperationBellCut            = "bell_cut"
	OperationEQBandPatch        = "eq.band.patch"
	OperationEQPassFilterPatch  = "eq.pass_filter.patch"
	OperationEQDynamicBandPatch = "eq.dynamic_band.patch"
	OperationEQOutputPatch      = "eq.output.patch"
)

// VPSStatus describes the learning/verification state of a complete VPS
// document. A document can contain candidate or stale Credentials while it is
// still a useful library record.
type VPSStatus string

const (
	VPSStatusDraft    VPSStatus = "draft"
	VPSStatusMapped   VPSStatus = "mapped"
	VPSStatusVerified VPSStatus = "verified"
	VPSStatusStale    VPSStatus = "stale"
	VPSStatusRevoked  VPSStatus = "revoked"
)

// CredentialStatus is intentionally separate from VPSStatus. A Credential is
// the dispatchable capability token; a VPS is the broader learned plugin
// document that can outlive a stale or revoked Credential.
type CredentialStatus string

const (
	CredentialDraft                  CredentialStatus = "draft"
	CredentialCandidate              CredentialStatus = "candidate"
	CredentialVerified               CredentialStatus = "verified"
	CredentialStale                  CredentialStatus = "stale"
	CredentialRevoked                CredentialStatus = "revoked"
	CredentialPendingRequalification CredentialStatus = "pending_requalification"
)

// CanTransitionTo defines the credential lifecycle. In particular, a stale or
// pending-requalification credential cannot jump straight back to verified;
// a fresh candidate/conformance revision is required.
func (s CredentialStatus) CanTransitionTo(next CredentialStatus) bool {
	switch s {
	case CredentialDraft:
		return next == CredentialDraft || next == CredentialCandidate || next == CredentialPendingRequalification || next == CredentialRevoked
	case CredentialCandidate:
		return next == CredentialCandidate || next == CredentialVerified || next == CredentialStale || next == CredentialRevoked
	case CredentialPendingRequalification:
		return next == CredentialPendingRequalification || next == CredentialCandidate || next == CredentialStale || next == CredentialRevoked
	case CredentialVerified:
		return next == CredentialVerified || next == CredentialStale || next == CredentialRevoked
	case CredentialStale:
		return next == CredentialStale || next == CredentialCandidate || next == CredentialRevoked
	case CredentialRevoked:
		return next == CredentialRevoked
	default:
		return false
	}
}

// PluginFingerprint records the three independently invalidating signatures
// required by ADR-SPAL-001. LegacyParameterSignature is retained only as
// migration evidence; it is never sufficient for a Verified Credential.
type PluginFingerprint struct {
	Installation             string `json:"installation,omitempty"`
	ParameterSurface         string `json:"parameter_surface,omitempty"`
	DisplaySurface           string `json:"display_surface,omitempty"`
	LegacyParameterSignature string `json:"legacy_parameter_signature,omitempty"`
}

// HostProjectionBinding preserves the boundary between the full plug-in
// surface captured by an independent adapter and a particular host's
// controllable projection.  It is a transport guard only: it never assigns a
// semantic meaning to a parameter, and its observed fingerprint does not
// replace PluginIdentity.Fingerprint.
//
// More than one binding may exist because the same locally installed VST3 can
// be hosted by more than one application.  Runtime code must select the host
// it is actually using instead of treating a Vit projection as a universal
// plug-in fingerprint.
type HostProjectionBinding struct {
	SchemaVersion        string            `json:"schema_version"`
	HostID               string            `json:"host_id"`
	Fingerprint          PluginFingerprint `json:"fingerprint"`
	RequiredParameterIDs []string          `json:"required_parameter_ids,omitempty"`
	EvidenceRefs         []string          `json:"evidence_refs,omitempty"`
	Trust                string            `json:"trust"`
	Status               string            `json:"status"`
	CapturedAt           time.Time         `json:"captured_at,omitempty"`
}

const (
	HostProjectionBindingSchemaVersion = "vit.vps.host_projection_binding.v1"
	// VitHostProjectionID names Vit's runtime-controllable parameter
	// projection.  It is intentionally distinct from the independent VST3
	// adapter surface held by PluginIdentity.Fingerprint.
	VitHostProjectionID = "vit.native_host.v1"
)

func (b HostProjectionBinding) Validate() error {
	if strings.TrimSpace(b.SchemaVersion) != HostProjectionBindingSchemaVersion {
		return fmt.Errorf("host projection binding has unsupported schema %q", b.SchemaVersion)
	}
	if strings.TrimSpace(b.HostID) == "" {
		return fmt.Errorf("host projection binding requires host_id")
	}
	if !b.Fingerprint.Complete() {
		return fmt.Errorf("host projection binding %s requires a complete fingerprint", b.HostID)
	}
	if strings.TrimSpace(b.Trust) != "observed" {
		return fmt.Errorf("host projection binding %s must remain observed", b.HostID)
	}
	if strings.TrimSpace(b.Status) != "observed_host_projection_compatible" {
		return fmt.Errorf("host projection binding %s is not compatible", b.HostID)
	}
	seen := map[string]bool{}
	for _, id := range b.RequiredParameterIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return fmt.Errorf("host projection binding %s has an empty or duplicate required parameter id", b.HostID)
		}
		seen[id] = true
	}
	if len(uniqueSorted(b.EvidenceRefs)) == 0 {
		return fmt.Errorf("host projection binding %s requires evidence refs", b.HostID)
	}
	return nil
}

// Complete reports whether the fingerprint is strong enough to support a
// v3 Verified Credential.
func (f PluginFingerprint) Complete() bool {
	return strings.TrimSpace(f.Installation) != "" &&
		strings.TrimSpace(f.ParameterSurface) != "" &&
		strings.TrimSpace(f.DisplaySurface) != ""
}

// Equal compares the dispatch-relevant fingerprint fields. Legacy evidence is
// deliberately excluded: it must not hide a real parameter/display change.
func (f PluginFingerprint) Equal(other PluginFingerprint) bool {
	return strings.TrimSpace(f.Installation) == strings.TrimSpace(other.Installation) &&
		strings.TrimSpace(f.ParameterSurface) == strings.TrimSpace(other.ParameterSurface) &&
		strings.TrimSpace(f.DisplaySurface) == strings.TrimSpace(other.DisplaySurface)
}

// PluginIdentity is vendor/plugin identity shared by all revisions of a VPS.
// It has no project, track or loaded-plugin-instance identifiers.
type PluginIdentity struct {
	Manufacturer string            `json:"manufacturer,omitempty"`
	Name         string            `json:"name,omitempty"`
	Format       string            `json:"format,omitempty"`
	Version      string            `json:"version,omitempty"`
	ProfileKey   string            `json:"profile_key,omitempty"`
	InstallPath  string            `json:"install_path,omitempty"`
	Fingerprint  PluginFingerprint `json:"fingerprint,omitempty"`
}

// InventoryStatus is intentionally independent of VPS/Credential status. An
// installed plug-in can be discovered while remaining completely unusable by
// SPAL until a VPS and Credential are created through learning/conformance.
type InventoryStatus string

const (
	InventoryStatusDiscovered InventoryStatus = "discovered"
	InventoryStatusMissing    InventoryStatus = "missing"
)

// PluginInventoryEntry is the user-level discovery record for an installed
// plugin. It does not contain a project instance and grants no dispatch right.
type PluginInventoryEntry struct {
	ID        string          `json:"id"`
	Identity  PluginIdentity  `json:"plugin_identity"`
	Status    InventoryStatus `json:"status"`
	Source    string          `json:"source,omitempty"`
	FirstSeen time.Time       `json:"first_seen"`
	LastSeen  time.Time       `json:"last_seen"`
}

func NewInventoryEntry(identity PluginIdentity, source string, now time.Time) PluginInventoryEntry {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return PluginInventoryEntry{
		ID:        "inventory_" + strings.TrimPrefix(NewDocumentID(identity), "vps_"),
		Identity:  identity,
		Status:    InventoryStatusDiscovered,
		Source:    strings.TrimSpace(source),
		FirstSeen: now.UTC(),
		LastSeen:  now.UTC(),
	}
}

func (e PluginInventoryEntry) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("plugin inventory entry requires id")
	}
	if err := e.Identity.validDraft(); err != nil {
		return err
	}
	switch e.Status {
	case InventoryStatusDiscovered, InventoryStatusMissing:
	default:
		return fmt.Errorf("plugin inventory entry %s has unknown status %q", e.ID, e.Status)
	}
	return nil
}

func (i PluginIdentity) validDraft() error {
	if strings.TrimSpace(i.Name) == "" && strings.TrimSpace(i.ProfileKey) == "" {
		return fmt.Errorf("VPS plugin identity requires a name or profile_key")
	}
	return nil
}

func (i PluginIdentity) validForCredential() error {
	if err := i.validDraft(); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"manufacturer", i.Manufacturer},
		{"name", i.Name},
		{"format", i.Format},
		{"version", i.Version},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("verified VPS plugin identity requires %s", field.name)
		}
	}
	if !i.Fingerprint.Complete() {
		return fmt.Errorf("verified VPS plugin identity requires installation, parameter-surface and display-surface fingerprints")
	}
	return nil
}

// RawObservation preserves a source fact from discovery, host parameter
// enumeration, documentation, a GUI demonstration or a legacy import.
type RawObservation struct {
	Kind       string          `json:"kind"`
	Source     string          `json:"source"`
	Summary    string          `json:"summary,omitempty"`
	CapturedAt time.Time       `json:"captured_at,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// DisplayDomain records the user-visible control domain restored from a host
// parameter/control mapping.
type DisplayDomain struct {
	Text  string   `json:"text,omitempty"`
	Unit  string   `json:"unit,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Scale string   `json:"scale,omitempty"`
}

// ControlSurfaceMapping links a semantic/plugin GUI control to a raw host
// parameter. These mappings remain evidence-backed and are not automatically
// dispatchable merely because they exist in a VPS.
type ControlSurfaceMapping struct {
	ComponentID   string        `json:"component_id,omitempty"`
	SemanticSlot  string        `json:"semantic_slot,omitempty"`
	ParameterID   string        `json:"parameter_id"`
	Label         string        `json:"label,omitempty"`
	DisplayDomain DisplayDomain `json:"display_domain,omitempty"`
	Transform     string        `json:"transform,omitempty"`
	EnumValues    []string      `json:"enum_values,omitempty"`
	Confirmed     bool          `json:"confirmed,omitempty"`
	// BindingStatus and ExecutionScope retain the provenance boundary for
	// manually assembled draft mappings.  They are intentionally descriptive:
	// neither field can make a mapping dispatchable or equivalent to a
	// Credential-backed binding.
	BindingStatus  string   `json:"binding_status,omitempty"`
	ExecutionScope string   `json:"execution_scope,omitempty"`
	Warning        string   `json:"warning,omitempty"`
	EvidenceRefs   []string `json:"evidence_refs,omitempty"`
}

// MacroRelation is a learned multi-parameter/virtual-control relationship.
// It lives beside display/control mappings so VPS remains the one plugin
// control-surface archive rather than splitting macro knowledge into another
// manually maintained file.
type MacroRelation struct {
	ID          string          `json:"id"`
	Inputs      []string        `json:"inputs,omitempty"`
	Resolver    string          `json:"resolver,omitempty"`
	ComponentID string          `json:"component_id,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type ControlSurface struct {
	Mappings       []ControlSurfaceMapping `json:"mappings,omitempty"`
	MacroRelations []MacroRelation         `json:"macro_relations,omitempty"`
}

// TopologyComponent and PhysicalResource describe plugin structure separately
// from a project resource lease. The latter will be introduced in the SPAL
// resource-execution phase.
type TopologyComponent struct {
	ID    string   `json:"id"`
	Role  string   `json:"role,omitempty"`
	Label string   `json:"label,omitempty"`
	Slots []string `json:"slots,omitempty"`
}

type PhysicalResource struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	ComponentID string   `json:"component_id,omitempty"`
	Capacity    int      `json:"capacity,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
}

type Topology struct {
	Components []TopologyComponent `json:"components,omitempty"`
	Resources  []PhysicalResource  `json:"resources,omitempty"`
}

// SemanticCapability describes a learned candidate capability. Only a
// separate ProviderCredential can make the candidate dispatchable.
type SemanticCapability struct {
	ID           string   `json:"id"`
	Operations   []string `json:"operations,omitempty"`
	Parameters   []string `json:"parameters,omitempty"`
	FilterTypes  []string `json:"filter_types,omitempty"`
	Status       string   `json:"status,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// SpecialCapability is an informational VPS extension. It is never made
// dispatchable by appearing in this list; a dedicated schema, conformance and
// Credential entry are still required. The digest may be shown to an Agent so
// it can answer an explicit user question about a vendor-specific feature.
type SpecialCapability struct {
	ID               string   `json:"id"`
	Status           string   `json:"status"`
	Dispatchable     bool     `json:"dispatchable"`
	InvocationPolicy string   `json:"invocation_policy,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	Unknowns         []string `json:"unknowns,omitempty"`
}

// SafetyAndRollback documents the safety/restore contract learned for a VPS.
type SafetyAndRollback struct {
	Preconditions []string `json:"preconditions,omitempty"`
	Bounds        []string `json:"bounds,omitempty"`
	RollbackMode  string   `json:"rollback_mode,omitempty"`
	Invalidators  []string `json:"invalidators,omitempty"`
	// LegacyPolicy preserves a source policy object during migration without
	// treating it as a verified v3 safety contract.
	LegacyPolicy json.RawMessage `json:"legacy_policy,omitempty"`
}

// FXMMeasurementRequirement keeps effect-projection work in the VPS authoring
// record without embedding raw audio or analyzer payloads in the VPS Library.
// Large renders and curves remain external artifacts referenced by EvidenceRefs.
type FXMMeasurementRequirement struct {
	SchemaVersion   string   `json:"schema_version"`
	ID              string   `json:"id"`
	ProbeSuiteID    string   `json:"probe_suite_id"`
	CapabilityID    string   `json:"capability_id,omitempty"`
	Operations      []string `json:"operations,omitempty"`
	RequiredStages  []string `json:"required_stages"`
	Metrics         []string `json:"metrics"`
	TestSignals     []string `json:"test_signals,omitempty"`
	ParameterStates []string `json:"parameter_states,omitempty"`
	BaselinePolicy  string   `json:"baseline_policy"`
	Status          string   `json:"status"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
	Limitations     []string `json:"limitations,omitempty"`
}

func (r FXMMeasurementRequirement) Validate() error {
	if r.SchemaVersion != FXMMeasurementRequirementSchema {
		return fmt.Errorf("FXM measurement requirement %s has unsupported schema %q", r.ID, r.SchemaVersion)
	}
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.BaselinePolicy) == "" {
		return fmt.Errorf("FXM measurement requirement requires id and baseline_policy")
	}
	if strings.TrimSpace(r.ProbeSuiteID) == "" {
		return fmt.Errorf("FXM measurement requirement %s requires probe_suite_id", r.ID)
	}
	if len(uniqueSorted(r.RequiredStages)) < 2 || !containsAll(r.RequiredStages, []string{"bypass_chain", "processed_chain"}) {
		return fmt.Errorf("FXM measurement requirement %s requires bypass_chain and processed_chain stages", r.ID)
	}
	if len(uniqueSorted(r.Metrics)) == 0 {
		return fmt.Errorf("FXM measurement requirement %s requires at least one metric", r.ID)
	}
	switch strings.TrimSpace(r.Status) {
	case "planned", "observed", "conformed", "unsupported":
	default:
		return fmt.Errorf("FXM measurement requirement %s has unknown status %q", r.ID, r.Status)
	}
	if r.Status == "conformed" && len(uniqueSorted(r.EvidenceRefs)) == 0 {
		return fmt.Errorf("conformed FXM measurement requirement %s requires evidence refs", r.ID)
	}
	return nil
}

// DefaultFXMMeasurementRequirement supplies the reproducible minimum expected
// from every newly authored effect VPS. Capability-specific authoring tools may
// extend the metrics and parameter states, but should not weaken this baseline.
func DefaultFXMMeasurementRequirement(capabilityID string) FXMMeasurementRequirement {
	capabilityID = strings.TrimSpace(capabilityID)
	idSuffix := strings.NewReplacer(".", "_", "/", "_", " ", "_").Replace(capabilityID)
	if idSuffix == "" {
		idSuffix = "plugin_effect"
	}
	return FXMMeasurementRequirement{
		SchemaVersion:   FXMMeasurementRequirementSchema,
		ID:              "fxm_" + idSuffix + "_baseline_v0",
		ProbeSuiteID:    "vit.vps_probe_audio.v1@48000",
		CapabilityID:    capabilityID,
		RequiredStages:  []string{"bypass_chain", "processed_chain"},
		Metrics:         []string{"rms_dbfs", "peak_dbfs", "band_energy_db", "latency_samples"},
		TestSignals:     []string{"impulse_stereo", "log_sweep_20hz_20khz", "multitone_31band", "stepped_sine_levels", "transient_burst", "stereo_phase_probe", "tail_probe", "deterministic_program_material"},
		ParameterStates: []string{"factory_or_documented_default", "representative_operation_state"},
		BaselinePolicy:  "same_source_window_routing_gain_automation_and_render_settings",
		Status:          "planned",
		Limitations:     []string{"FXM is evidence for transformation, not a musical-quality claim", "dynamic and nonlinear behavior remains conditional on input and measurement window"},
	}
}

// CapabilityConformanceProfile defines the finite, versioned requirements of
// one semantic capability. It is data, rather than a vendor-specific adapter.
type CapabilityConformanceProfile struct {
	ID                            string   `json:"id"`
	Version                       string   `json:"version"`
	RequiredOperations            []string `json:"required_operations"`
	RequiredParameters            []string `json:"required_parameters"`
	RequiredFilterTypes           []string `json:"required_filter_types,omitempty"`
	RequiredSchemas               []string `json:"required_schemas,omitempty"`
	RequiresWriteReadback         bool     `json:"requires_write_readback"`
	RequiresBoundaryTests         bool     `json:"requires_boundary_tests"`
	RequiresRollbackTest          bool     `json:"requires_rollback_test"`
	RequiresBehaviorTest          bool     `json:"requires_behavior_test,omitempty"`
	RequiresStateRetentionTest    bool     `json:"requires_state_transition_retention_test,omitempty"`
	RequiresResourceIsolationTest bool     `json:"requires_resource_isolation_test,omitempty"`
}

// StaticEQConformanceProfileV0 is the first capability profile mandated by
// ADR-SPAL-001. Providers may support more filter types, but bell is required
// for the initial bell_cut semantic operation.
func StaticEQConformanceProfileV0() CapabilityConformanceProfile {
	return CapabilityConformanceProfile{
		ID:                    StaticEQCapabilityID,
		Version:               StaticEQProfileVersion,
		RequiredOperations:    []string{OperationAllocateBand, OperationPatchBand, OperationReadBand, OperationReleaseBand, OperationBellCut},
		RequiredParameters:    []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"},
		RequiredFilterTypes:   []string{"bell"},
		RequiresWriteReadback: true,
		RequiresBoundaryTests: true,
		RequiresRollbackTest:  true,
	}
}

// EqualizerConformanceProfileV2 is the work-card admission contract.  Visual
// analysis is intentionally not required.  Vendor-specific modes such as
// Sticky/W-Band/presets are outside these generic schemas.
func EqualizerConformanceProfileV2() CapabilityConformanceProfile {
	return CapabilityConformanceProfile{
		ID: EqualizerCapabilityID, Version: EqualizerProfileVersion,
		RequiredOperations:    []string{OperationEQBandPatch},
		RequiredParameters:    []string{"band_ref", "enabled", "response_shape", "frequency_hz", "gain_db", "q"},
		RequiredFilterTypes:   []string{"bell"},
		RequiredSchemas:       []string{spal.EQBandPatchControlID},
		RequiresWriteReadback: true, RequiresBoundaryTests: true, RequiresRollbackTest: true,
		RequiresBehaviorTest: true, RequiresStateRetentionTest: true, RequiresResourceIsolationTest: true,
	}
}

func (p CapabilityConformanceProfile) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("capability conformance profile requires id and version")
	}
	if len(uniqueSorted(p.RequiredOperations)) != len(p.RequiredOperations) || len(p.RequiredOperations) == 0 {
		return fmt.Errorf("capability conformance profile requires unique operations")
	}
	if len(uniqueSorted(p.RequiredParameters)) != len(p.RequiredParameters) || len(p.RequiredParameters) == 0 {
		return fmt.Errorf("capability conformance profile requires unique parameters")
	}
	if p.ID == StaticEQCapabilityID {
		want := StaticEQConformanceProfileV0()
		if p.Version != want.Version || !sameStrings(p.RequiredOperations, want.RequiredOperations) || !sameStrings(p.RequiredParameters, want.RequiredParameters) || !sameStrings(p.RequiredFilterTypes, want.RequiredFilterTypes) || !p.RequiresWriteReadback || !p.RequiresBoundaryTests || !p.RequiresRollbackTest {
			return fmt.Errorf("%s conformance profile does not satisfy v0 requirements", StaticEQCapabilityID)
		}
	}
	if p.ID == EqualizerCapabilityID {
		want := EqualizerConformanceProfileV2()
		if p.Version != want.Version || !sameStrings(p.RequiredOperations, want.RequiredOperations) || !sameStrings(p.RequiredParameters, want.RequiredParameters) || !sameStrings(p.RequiredFilterTypes, want.RequiredFilterTypes) || !sameStrings(p.RequiredSchemas, want.RequiredSchemas) ||
			!p.RequiresWriteReadback || !p.RequiresBoundaryTests || !p.RequiresRollbackTest || !p.RequiresBehaviorTest || !p.RequiresStateRetentionTest || !p.RequiresResourceIsolationTest {
			return fmt.Errorf("%s conformance profile does not satisfy v2 work-card requirements", EqualizerCapabilityID)
		}
	}
	return nil
}

// CredentialConformance is the durable evidence that one Credential meets a
// capability profile. It contains only evidence refs, not project binding.
type CredentialConformance struct {
	ProfileID               string            `json:"profile_id"`
	ProfileVersion          string            `json:"profile_version"`
	Operations              []string          `json:"operations,omitempty"`
	Parameters              []string          `json:"parameters,omitempty"`
	FilterTypes             []string          `json:"filter_types,omitempty"`
	StaticEQBinding         *StaticEQBinding  `json:"static_eq_binding,omitempty"`
	EQV2Binding             *spal.EQV2Binding `json:"eq_v2_binding,omitempty"`
	ConformedSchemas        []string          `json:"conformed_schemas,omitempty"`
	WriteReadbackPassed     bool              `json:"write_readback_passed,omitempty"`
	BoundaryTestsPassed     bool              `json:"boundary_tests_passed,omitempty"`
	RollbackTestPassed      bool              `json:"rollback_test_passed,omitempty"`
	BehaviorTestsPassed     bool              `json:"behavior_tests_passed,omitempty"`
	BehaviorScope           string            `json:"behavior_scope,omitempty"`
	StateRetentionPassed    bool              `json:"state_retention_passed,omitempty"`
	ResourceIsolationPassed bool              `json:"resource_isolation_passed,omitempty"`
	EvidenceRefs            []string          `json:"evidence_refs,omitempty"`
	CompletedAt             time.Time         `json:"completed_at,omitempty"`
}

func (c CredentialConformance) validates(profile CapabilityConformanceProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.ProfileID) != profile.ID || strings.TrimSpace(c.ProfileVersion) != profile.Version {
		return fmt.Errorf("credential conformance does not match profile %s@%s", profile.ID, profile.Version)
	}
	if !containsAll(c.Operations, profile.RequiredOperations) {
		return fmt.Errorf("credential conformance omits required operations")
	}
	if !containsAll(c.Parameters, profile.RequiredParameters) {
		return fmt.Errorf("credential conformance omits required parameters")
	}
	if !containsAll(c.FilterTypes, profile.RequiredFilterTypes) {
		return fmt.Errorf("credential conformance omits required filter types")
	}
	if profile.ID == StaticEQCapabilityID {
		if c.StaticEQBinding == nil {
			return fmt.Errorf("credential conformance omits the persisted static EQ binding")
		}
		if err := c.StaticEQBinding.Validate(); err != nil {
			return fmt.Errorf("credential conformance static EQ binding: %w", err)
		}
	}
	if profile.ID == EqualizerCapabilityID {
		if c.EQV2Binding == nil {
			return fmt.Errorf("credential conformance omits the persisted EQ v2 binding")
		}
		if !containsAll(c.ConformedSchemas, profile.RequiredSchemas) || !containsAll(c.EQV2Binding.ConformedSchemas, profile.RequiredSchemas) {
			return fmt.Errorf("credential conformance omits required EQ v2 schemas")
		}
		definition := spal.EQV2ProviderDefinition{
			Descriptor: spal.ProviderDescriptor{ID: "validation", AdapterVersion: "v2", Status: spal.ProviderVerified, SupportedSchemas: append([]string(nil), c.ConformedSchemas...)},
			VPSID:      "validation", CredentialID: "validation", Binding: *c.EQV2Binding,
		}
		if _, err := spal.NewVPSEQV2Adapter(definition); err != nil {
			return fmt.Errorf("credential conformance EQ v2 binding: %w", err)
		}
	}
	if profile.RequiresWriteReadback && !c.WriteReadbackPassed {
		return fmt.Errorf("credential conformance has not passed write/readback")
	}
	if profile.RequiresBoundaryTests && !c.BoundaryTestsPassed {
		return fmt.Errorf("credential conformance has not passed boundary tests")
	}
	if profile.RequiresRollbackTest && !c.RollbackTestPassed {
		return fmt.Errorf("credential conformance has not passed rollback test")
	}
	if profile.RequiresBehaviorTest && !c.BehaviorTestsPassed {
		return fmt.Errorf("credential conformance has not passed behavior tests")
	}
	if profile.RequiresStateRetentionTest && !c.StateRetentionPassed {
		return fmt.Errorf("credential conformance has not passed state-retention tests")
	}
	if profile.RequiresResourceIsolationTest && !c.ResourceIsolationPassed {
		return fmt.Errorf("credential conformance has not passed resource-isolation tests")
	}
	if len(uniqueSorted(c.EvidenceRefs)) == 0 {
		return fmt.Errorf("credential conformance requires evidence refs")
	}
	if c.CompletedAt.IsZero() {
		return fmt.Errorf("credential conformance requires a completion timestamp")
	}
	return nil
}

// ProviderCredential is a reusable capability token stored inside a VPS. It
// never names a project, track, loaded plugin or physical resource lease.
type ProviderCredential struct {
	ID                 string                `json:"id"`
	Revision           int                   `json:"revision"`
	CapabilityID       string                `json:"capability_id"`
	Status             CredentialStatus      `json:"status"`
	PluginFingerprint  PluginFingerprint     `json:"plugin_fingerprint,omitempty"`
	Conformance        CredentialConformance `json:"conformance,omitempty"`
	Safety             SafetyAndRollback     `json:"safety,omitempty"`
	EvidenceRefs       []string              `json:"evidence_refs,omitempty"`
	IssuedAt           time.Time             `json:"issued_at,omitempty"`
	UpdatedAt          time.Time             `json:"updated_at,omitempty"`
	InvalidatedAt      *time.Time            `json:"invalidated_at,omitempty"`
	InvalidationReason string                `json:"invalidation_reason,omitempty"`
	MigrationSource    string                `json:"migration_source,omitempty"`
}

func (c ProviderCredential) Dispatchable() bool {
	return c.Status == CredentialVerified
}

func (c ProviderCredential) validate(identity PluginIdentity) error {
	if err := c.validateBasic(); err != nil {
		return err
	}
	if c.Status != CredentialVerified {
		return nil
	}
	profile, ok := ProfileFor(c.CapabilityID)
	if !ok {
		return fmt.Errorf("verified credential %s uses an unknown capability %q", c.ID, c.CapabilityID)
	}
	return c.validateWithProfile(identity, profile)
}

func (c ProviderCredential) validateBasic() error {
	if strings.TrimSpace(c.ID) == "" || c.Revision < 1 || strings.TrimSpace(c.CapabilityID) == "" {
		return fmt.Errorf("provider credential requires id, positive revision and capability_id")
	}
	switch c.Status {
	case CredentialDraft, CredentialCandidate, CredentialVerified, CredentialStale, CredentialRevoked, CredentialPendingRequalification:
	default:
		return fmt.Errorf("provider credential %s has unknown status %q", c.ID, c.Status)
	}
	return nil
}

func (c ProviderCredential) validateWithProfile(identity PluginIdentity, profile CapabilityConformanceProfile) error {
	if err := c.validateBasic(); err != nil {
		return err
	}
	if c.Status != CredentialVerified {
		return nil
	}
	if err := identity.validForCredential(); err != nil {
		return err
	}
	if !c.PluginFingerprint.Complete() || !c.PluginFingerprint.Equal(identity.Fingerprint) {
		return fmt.Errorf("verified credential %s does not match a complete VPS plugin fingerprint", c.ID)
	}
	if err := c.Conformance.validates(profile); err != nil {
		return fmt.Errorf("verified credential %s: %w", c.ID, err)
	}
	return nil
}

func validateCredentialTransition(previous, next ProviderCredential) error {
	if !previous.Status.CanTransitionTo(next.Status) {
		return fmt.Errorf("credential %s cannot transition from %s to %s", previous.ID, previous.Status, next.Status)
	}
	if next.Revision < previous.Revision {
		return fmt.Errorf("credential %s revision cannot decrease", previous.ID)
	}
	if (previous.Status == CredentialStale || previous.Status == CredentialPendingRequalification) && next.Status == CredentialCandidate && next.Revision <= previous.Revision {
		return fmt.Errorf("credential %s requires a new revision for requalification", previous.ID)
	}
	return nil
}

// CredentialStateChange is returned when a fresh plugin fingerprint invalidates
// previously verified credential(s). Stale credentials never auto-recover.
type CredentialStateChange struct {
	CredentialID string
	From         CredentialStatus
	To           CredentialStatus
	Reason       string
}

// MigrationMetadata makes legacy origins explicit and prevents an imported
// artifact from gaining dispatch authority merely by being persisted.
type MigrationMetadata struct {
	Sources                 []string  `json:"sources,omitempty"`
	LegacyIDs               []string  `json:"legacy_ids,omitempty"`
	RequiresRequalification bool      `json:"requires_requalification,omitempty"`
	ImportedAt              time.Time `json:"imported_at,omitempty"`
}

// VPSDocument is the canonical v3 plugin specification archive.
type VPSDocument struct {
	SchemaVersion        string                         `json:"schema_version"`
	ID                   string                         `json:"id"`
	Revision             int                            `json:"revision"`
	Status               VPSStatus                      `json:"status"`
	PluginIdentity       PluginIdentity                 `json:"plugin_identity"`
	RawObservations      []RawObservation               `json:"raw_observations,omitempty"`
	ControlSurface       ControlSurface                 `json:"control_surface,omitempty"`
	Topology             Topology                       `json:"topology,omitempty"`
	SemanticCapabilities []SemanticCapability           `json:"semantic_capabilities,omitempty"`
	SpecialCapabilities  []SpecialCapability            `json:"special_capabilities,omitempty"`
	CapabilityProfiles   []CapabilityConformanceProfile `json:"capability_conformance_profiles,omitempty"`
	// HostProjectionBindings are host-specific observed transport guards. They
	// deliberately live outside PluginIdentity.Fingerprint, which remains the
	// independent full plug-in identity used by a Credential.
	HostProjectionBindings []HostProjectionBinding `json:"host_projection_bindings,omitempty"`
	// BadgeActionImplementations is the plug-in-specific realization of the
	// fixed task-badge action grammar.  It is deliberately separate from
	// observed control mappings: a parameter ID becomes executable only when
	// it appears in one of these reviewed action implementations.
	BadgeActionImplementations []VPSActionImplementation   `json:"badge_action_implementations,omitempty"`
	ProviderCredentials        []ProviderCredential        `json:"provider_credentials,omitempty"`
	ConformanceEvidence        []string                    `json:"conformance_evidence,omitempty"`
	SafetyAndRollback          SafetyAndRollback           `json:"safety_and_rollback,omitempty"`
	FXMMeasurements            []FXMMeasurementRequirement `json:"fxm_measurements,omitempty"`
	Migration                  MigrationMetadata           `json:"migration,omitempty"`
	CreatedAt                  time.Time                   `json:"created_at"`
	UpdatedAt                  time.Time                   `json:"updated_at"`
}

// NewDraft creates an unverified VPS suitable for Plugin Learning imports.
func NewDraft(identity PluginIdentity, now time.Time) VPSDocument {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return VPSDocument{
		SchemaVersion:  DocumentSchemaVersion,
		ID:             NewDocumentID(identity),
		Revision:       1,
		Status:         VPSStatusDraft,
		PluginIdentity: identity,
		CreatedAt:      now.UTC(),
		UpdatedAt:      now.UTC(),
	}
}

// NewDocumentID creates a stable ID from user-level plugin identity. Callers
// can replace it for a deliberately separate migration record.
func NewDocumentID(identity PluginIdentity) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(identity.Manufacturer)),
		strings.ToLower(strings.TrimSpace(identity.Name)),
		strings.ToLower(strings.TrimSpace(identity.Format)),
		strings.TrimSpace(identity.Version),
		strings.TrimSpace(identity.ProfileKey),
		strings.TrimSpace(identity.Fingerprint.Installation),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "vps_" + hex.EncodeToString(sum[:])[:20]
}

// Validate checks archive consistency. It intentionally permits incomplete
// drafts and pending-requalification credentials, but never a malformed or
// implicitly dispatchable document.
func (d VPSDocument) Validate() error {
	if d.SchemaVersion != DocumentSchemaVersion {
		return fmt.Errorf("unsupported VPS schema_version %q", d.SchemaVersion)
	}
	if strings.TrimSpace(d.ID) == "" || d.Revision < 1 {
		return fmt.Errorf("VPS requires id and positive revision")
	}
	if err := d.PluginIdentity.validDraft(); err != nil {
		return err
	}
	switch d.Status {
	case VPSStatusDraft, VPSStatusMapped, VPSStatusVerified, VPSStatusStale, VPSStatusRevoked:
	default:
		return fmt.Errorf("VPS %s has unknown status %q", d.ID, d.Status)
	}
	capabilities := map[string]bool{}
	for _, capability := range d.SemanticCapabilities {
		id := strings.TrimSpace(capability.ID)
		if id == "" {
			return fmt.Errorf("VPS %s contains a semantic capability without id", d.ID)
		}
		if capabilities[id] {
			return fmt.Errorf("VPS %s repeats semantic capability %s", d.ID, id)
		}
		capabilities[id] = true
	}
	profiles := map[string]CapabilityConformanceProfile{}
	for _, profile := range d.CapabilityProfiles {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("VPS %s: %w", d.ID, err)
		}
		if _, exists := profiles[profile.ID]; exists {
			return fmt.Errorf("VPS %s repeats capability profile %s", d.ID, profile.ID)
		}
		profiles[profile.ID] = profile
	}
	implementations := map[string]VPSActionImplementation{}
	for _, implementation := range d.BadgeActionImplementations {
		contract, known := BadgeContractFor(implementation.BadgeID)
		if !known {
			return fmt.Errorf("VPS %s action implementation uses unknown task badge %s", d.ID, implementation.BadgeID)
		}
		if err := implementation.ValidateAgainst(contract); err != nil {
			return fmt.Errorf("VPS %s: %w", d.ID, err)
		}
		key := strings.TrimSpace(implementation.BadgeID) + "\x00" + strings.TrimSpace(implementation.ActionID)
		if _, exists := implementations[key]; exists {
			return fmt.Errorf("VPS %s repeats task-badge action implementation %s", d.ID, implementation.ActionID)
		}
		implementations[key] = implementation
	}
	hostBindings := map[string]bool{}
	for _, binding := range d.HostProjectionBindings {
		if err := binding.Validate(); err != nil {
			return fmt.Errorf("VPS %s: %w", d.ID, err)
		}
		hostID := strings.ToLower(strings.TrimSpace(binding.HostID))
		if hostBindings[hostID] {
			return fmt.Errorf("VPS %s repeats host projection binding %s", d.ID, binding.HostID)
		}
		hostBindings[hostID] = true
	}
	credentialIDs := map[string]bool{}
	hasVerified := false
	for _, credential := range d.ProviderCredentials {
		if credentialIDs[credential.ID] {
			return fmt.Errorf("VPS %s repeats credential id %s", d.ID, credential.ID)
		}
		credentialIDs[credential.ID] = true
		var err error
		if credential.Status == CredentialVerified {
			if !capabilities[credential.CapabilityID] {
				return fmt.Errorf("VPS %s verified credential %s is missing its semantic capability declaration", d.ID, credential.ID)
			}
			profile, ok := profiles[credential.CapabilityID]
			if !ok {
				return fmt.Errorf("VPS %s verified credential %s is missing its persisted capability profile", d.ID, credential.ID)
			}
			err = credential.validateWithProfile(d.PluginIdentity, profile)
			if err == nil && len(d.BadgeActionImplementations) > 0 {
				for _, schemaID := range credential.Conformance.ConformedSchemas {
					if !hasConformedBadgeActionForSchema(d.BadgeActionImplementations, credential.CapabilityID, schemaID) {
						err = fmt.Errorf("verified credential %s has schema %s without a conformed task-badge action implementation", credential.ID, schemaID)
						break
					}
				}
			}
		} else {
			err = credential.validateBasic()
		}
		if err != nil {
			return fmt.Errorf("VPS %s: %w", d.ID, err)
		}
		if credential.Status == CredentialVerified {
			hasVerified = true
		}
	}
	if d.Status == VPSStatusVerified && !hasVerified {
		return fmt.Errorf("verified VPS %s has no verified credential", d.ID)
	}
	if d.Migration.RequiresRequalification && hasVerified {
		return fmt.Errorf("migrated VPS %s cannot contain an automatically verified credential", d.ID)
	}
	if err := validateUniqueTopology(d.Topology); err != nil {
		return err
	}
	measurementIDs := map[string]bool{}
	for _, requirement := range d.FXMMeasurements {
		if err := requirement.Validate(); err != nil {
			return fmt.Errorf("VPS %s: %w", d.ID, err)
		}
		if measurementIDs[requirement.ID] {
			return fmt.Errorf("VPS %s repeats FXM measurement requirement %s", d.ID, requirement.ID)
		}
		measurementIDs[requirement.ID] = true
	}
	return nil
}

// HostProjectionBindingFor returns a copied, explicitly observed binding for
// one runtime host.  It intentionally does not fall back to an unrelated
// host binding: callers must either find their own host guard or use the
// legacy full-surface compatibility path.
func (d VPSDocument) HostProjectionBindingFor(hostID string) (HostProjectionBinding, bool) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return HostProjectionBinding{}, false
	}
	for _, binding := range d.HostProjectionBindings {
		if !strings.EqualFold(strings.TrimSpace(binding.HostID), hostID) {
			continue
		}
		copy := binding
		copy.RequiredParameterIDs = append([]string(nil), binding.RequiredParameterIDs...)
		copy.EvidenceRefs = append([]string(nil), binding.EvidenceRefs...)
		return copy, true
	}
	return HostProjectionBinding{}, false
}

func hasConformedBadgeActionForSchema(implementations []VPSActionImplementation, badgeID, schemaID string) bool {
	for _, implementation := range implementations {
		if strings.TrimSpace(implementation.BadgeID) == strings.TrimSpace(badgeID) &&
			strings.TrimSpace(implementation.SchemaID) == strings.TrimSpace(schemaID) &&
			implementation.Status == BadgeFeatureStatusConformed {
			return true
		}
	}
	return false
}

func validateUniqueTopology(topology Topology) error {
	components := map[string]bool{}
	for _, component := range topology.Components {
		id := strings.TrimSpace(component.ID)
		if id == "" {
			return fmt.Errorf("VPS topology contains a component without id")
		}
		if components[id] {
			return fmt.Errorf("VPS topology repeats component %s", id)
		}
		components[id] = true
	}
	resources := map[string]bool{}
	for _, resource := range topology.Resources {
		id := strings.TrimSpace(resource.ID)
		if id == "" || strings.TrimSpace(resource.Kind) == "" {
			return fmt.Errorf("VPS topology resource requires id and kind")
		}
		if resources[id] {
			return fmt.Errorf("VPS topology repeats resource %s", id)
		}
		resources[id] = true
	}
	return nil
}

// ProfileFor returns the capability contract known by this Foundation.
func ProfileFor(capabilityID string) (CapabilityConformanceProfile, bool) {
	switch strings.TrimSpace(capabilityID) {
	case StaticEQCapabilityID:
		return StaticEQConformanceProfileV0(), true
	case EqualizerCapabilityID:
		return EqualizerConformanceProfileV2(), true
	}
	return CapabilityConformanceProfile{}, false
}

// ReconcileCredentialFingerprints marks only currently Verified Credentials
// stale when a current plugin observation differs. It never attempts to
// promote, revalidate or revive a credential automatically.
func (d *VPSDocument) ReconcileCredentialFingerprints(current PluginFingerprint, now time.Time) ([]CredentialStateChange, error) {
	if d == nil {
		return nil, fmt.Errorf("VPS document is required")
	}
	if !current.Complete() {
		return nil, fmt.Errorf("current plugin fingerprint must include installation, parameter-surface and display-surface signatures")
	}
	at := now.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	changes := []CredentialStateChange{}
	for index := range d.ProviderCredentials {
		credential := &d.ProviderCredentials[index]
		if credential.Status != CredentialVerified {
			continue
		}
		if credential.PluginFingerprint.Equal(current) {
			continue
		}
		from := credential.Status
		credential.Status = CredentialStale
		credential.InvalidationReason = fingerprintMismatchReason(credential.PluginFingerprint, current)
		credential.InvalidatedAt = &at
		credential.UpdatedAt = at
		changes = append(changes, CredentialStateChange{CredentialID: credential.ID, From: from, To: credential.Status, Reason: credential.InvalidationReason})
	}
	if len(changes) > 0 {
		d.Status = VPSStatusStale
		d.UpdatedAt = at
	}
	return changes, nil
}

func fingerprintMismatchReason(expected, actual PluginFingerprint) string {
	switch {
	case strings.TrimSpace(expected.Installation) != strings.TrimSpace(actual.Installation):
		return "plugin installation fingerprint changed"
	case strings.TrimSpace(expected.ParameterSurface) != strings.TrimSpace(actual.ParameterSurface):
		return "plugin parameter-surface signature changed"
	default:
		return "plugin display-surface signature changed"
	}
}

// RevokeCredential permanently removes a Credential from dispatch eligibility.
func (d *VPSDocument) RevokeCredential(credentialID, reason string, now time.Time) error {
	if d == nil {
		return fmt.Errorf("VPS document is required")
	}
	credentialID = strings.TrimSpace(credentialID)
	reason = strings.TrimSpace(reason)
	if credentialID == "" || reason == "" {
		return fmt.Errorf("credential id and revocation reason are required")
	}
	for index := range d.ProviderCredentials {
		credential := &d.ProviderCredentials[index]
		if credential.ID != credentialID {
			continue
		}
		if credential.Status == CredentialRevoked {
			return nil
		}
		at := now.UTC()
		if at.IsZero() {
			at = time.Now().UTC()
		}
		credential.Status = CredentialRevoked
		credential.InvalidatedAt = &at
		credential.InvalidationReason = reason
		credential.UpdatedAt = at
		d.UpdatedAt = at
		if !d.hasDispatchableCredential() {
			d.Status = VPSStatusRevoked
		}
		return nil
	}
	return fmt.Errorf("credential %s is not in VPS %s", credentialID, d.ID)
}

// Requalify replaces the dispatch authority of a migrated VPS only after a
// later learning/conformance workflow has supplied a complete current
// fingerprint and fully verified Credential(s). It is intentionally explicit:
// importing a v0/v2 artifact can never call this method on its own.
func (d *VPSDocument) Requalify(identity PluginIdentity, credentials []ProviderCredential, now time.Time) error {
	if d == nil {
		return fmt.Errorf("VPS document is required")
	}
	if err := identity.validForCredential(); err != nil {
		return err
	}
	if len(credentials) == 0 {
		return fmt.Errorf("requalification requires at least one credential")
	}
	profiles := append([]CapabilityConformanceProfile(nil), d.CapabilityProfiles...)
	profileIDs := map[string]bool{}
	for _, profile := range profiles {
		profileIDs[profile.ID] = true
	}
	verified := false
	for _, credential := range credentials {
		if credential.Status == CredentialVerified {
			verified = true
			profile, ok := ProfileFor(credential.CapabilityID)
			if !ok {
				return fmt.Errorf("requalification has no known capability profile for %s", credential.CapabilityID)
			}
			if !profileIDs[profile.ID] {
				profiles = append(profiles, profile)
				profileIDs[profile.ID] = true
			}
		}
		profile, ok := ProfileFor(credential.CapabilityID)
		if credential.Status == CredentialVerified && !ok {
			return fmt.Errorf("requalification has no known capability profile for %s", credential.CapabilityID)
		}
		var err error
		if credential.Status == CredentialVerified {
			err = credential.validateWithProfile(identity, profile)
		} else {
			err = credential.validateBasic()
		}
		if err != nil {
			return err
		}
	}
	if !verified {
		return fmt.Errorf("requalification requires a verified credential")
	}
	at := now.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	d.PluginIdentity = identity
	d.ProviderCredentials = append([]ProviderCredential(nil), credentials...)
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	d.CapabilityProfiles = profiles
	d.ensureCredentialCapabilities(credentials)
	d.Migration.RequiresRequalification = false
	d.Status = VPSStatusVerified
	d.UpdatedAt = at
	return d.Validate()
}

func (d *VPSDocument) ensureCredentialCapabilities(credentials []ProviderCredential) {
	known := map[string]bool{}
	for _, capability := range d.SemanticCapabilities {
		known[capability.ID] = true
	}
	for _, credential := range credentials {
		if credential.Status != CredentialVerified || known[credential.CapabilityID] {
			continue
		}
		profile, ok := ProfileFor(credential.CapabilityID)
		if !ok {
			continue
		}
		d.SemanticCapabilities = append(d.SemanticCapabilities, SemanticCapability{
			ID:           profile.ID,
			Operations:   append([]string(nil), profile.RequiredOperations...),
			Parameters:   append([]string(nil), profile.RequiredParameters...),
			FilterTypes:  append([]string(nil), profile.RequiredFilterTypes...),
			Status:       string(CredentialVerified),
			EvidenceRefs: uniqueSorted(append(append([]string(nil), credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...)),
		})
		known[credential.CapabilityID] = true
	}
	sort.Slice(d.SemanticCapabilities, func(i, j int) bool { return d.SemanticCapabilities[i].ID < d.SemanticCapabilities[j].ID })
}

func (d VPSDocument) hasDispatchableCredential() bool {
	for _, credential := range d.ProviderCredentials {
		if credential.Dispatchable() {
			return true
		}
	}
	return false
}

// ProviderCatalog is a read model derived from current Verified Credentials.
// It is intentionally not persisted as a second source of truth.
type ProviderCatalog struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedAt   time.Time              `json:"generated_at"`
	Entries       []ProviderCatalogEntry `json:"entries"`
}

type ProviderCatalogEntry struct {
	CredentialID        string              `json:"credential_id"`
	CredentialRevision  int                 `json:"credential_revision"`
	VPSID               string              `json:"vps_id"`
	CapabilityID        string              `json:"capability_id"`
	PluginIdentity      PluginIdentity      `json:"plugin_identity"`
	Operations          []string            `json:"operations"`
	FilterTypes         []string            `json:"filter_types,omitempty"`
	SupportedSchemas    []string            `json:"supported_schemas,omitempty"`
	WorkCards           []string            `json:"work_cards,omitempty"`
	SpecialCapabilities []SpecialCapability `json:"special_capabilities,omitempty"`
	EvidenceRefs        []string            `json:"evidence_refs,omitempty"`
}

// DeriveProviderCatalog returns only valid, explicitly Verified Credentials.
func DeriveProviderCatalog(documents []VPSDocument, generatedAt time.Time) (ProviderCatalog, error) {
	catalog := ProviderCatalog{SchemaVersion: CatalogSchemaVersion, GeneratedAt: generatedAt.UTC(), Entries: []ProviderCatalogEntry{}}
	for _, document := range documents {
		if err := document.Validate(); err != nil {
			return ProviderCatalog{}, err
		}
		for _, credential := range document.ProviderCredentials {
			if !credential.Dispatchable() {
				continue
			}
			catalog.Entries = append(catalog.Entries, ProviderCatalogEntry{
				CredentialID:        credential.ID,
				CredentialRevision:  credential.Revision,
				VPSID:               document.ID,
				CapabilityID:        credential.CapabilityID,
				PluginIdentity:      document.PluginIdentity,
				Operations:          uniqueSorted(credential.Conformance.Operations),
				FilterTypes:         uniqueSorted(credential.Conformance.FilterTypes),
				SupportedSchemas:    uniqueSorted(credential.Conformance.ConformedSchemas),
				WorkCards:           credentialWorkCards(credential),
				SpecialCapabilities: safeSpecialCapabilityDigest(document.SpecialCapabilities),
				EvidenceRefs:        uniqueSorted(append(append([]string(nil), credential.EvidenceRefs...), credential.Conformance.EvidenceRefs...)),
			})
		}
	}
	sort.Slice(catalog.Entries, func(i, j int) bool {
		if catalog.Entries[i].CapabilityID == catalog.Entries[j].CapabilityID {
			if catalog.Entries[i].VPSID == catalog.Entries[j].VPSID {
				return catalog.Entries[i].CredentialID < catalog.Entries[j].CredentialID
			}
			return catalog.Entries[i].VPSID < catalog.Entries[j].VPSID
		}
		return catalog.Entries[i].CapabilityID < catalog.Entries[j].CapabilityID
	})
	return catalog, nil
}

func credentialWorkCards(credential ProviderCredential) []string {
	if credential.CapabilityID == EqualizerCapabilityID {
		return []string{"equalizer"}
	}
	return nil
}

func safeSpecialCapabilityDigest(values []SpecialCapability) []SpecialCapability {
	out := make([]SpecialCapability, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.ID) == "" {
			continue
		}
		value.Dispatchable = false
		if strings.TrimSpace(value.InvocationPolicy) == "" {
			value.InvocationPolicy = "user_initiated_only"
		}
		value.Unknowns = uniqueSorted(value.Unknowns)
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func containsAll(actual, required []string) bool {
	set := map[string]bool{}
	for _, item := range actual {
		if item = strings.TrimSpace(item); item != "" {
			set[item] = true
		}
	}
	for _, item := range required {
		if !set[strings.TrimSpace(item)] {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	left = uniqueSorted(left)
	right = uniqueSorted(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
