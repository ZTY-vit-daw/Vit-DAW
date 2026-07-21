package spal

import (
	"fmt"
	"strings"
)

// EQV2StagingProviderDefinition is an explicitly local, non-routeable use of
// the same EQ v2 compiler used by a verified Credential. ImplementedSchemas
// are staging authority only; they are deliberately kept separate from
// EQV2Binding.ConformedSchemas so a Forge workspace cannot accidentally turn
// into a Catalog provider merely by being loaded.
type EQV2StagingProviderDefinition struct {
	Descriptor         ProviderDescriptor `json:"descriptor"`
	VPSID              string             `json:"vps_id"`
	StagingID          string             `json:"staging_id"`
	Binding            EQV2Binding        `json:"binding"`
	ImplementedSchemas []string           `json:"implemented_schemas"`
}

// VPSEQV2StagingAdapter is a thin authority boundary around VPSEQV2Adapter.
// It reuses its schema validation and physical compiler, but accepts only a
// explicitly selected staging instance and never exposes a verified Provider
// descriptor to a caller.
type VPSEQV2StagingAdapter struct {
	definition EQV2StagingProviderDefinition
	compiler   *VPSEQV2Adapter
	compilerID string
}

func NewVPSEQV2StagingAdapter(definition EQV2StagingProviderDefinition) (*VPSEQV2StagingAdapter, error) {
	definition = cloneEQV2StagingProviderDefinition(definition)
	if strings.TrimSpace(definition.VPSID) == "" || strings.TrimSpace(definition.StagingID) == "" {
		return nil, fmt.Errorf("VPS EQ v2 staging definition requires VPS and staging IDs")
	}
	if err := definition.Descriptor.Valid(); err != nil {
		return nil, err
	}
	if definition.Descriptor.Status != ProviderCandidate || !definition.Descriptor.Experimental {
		return nil, fmt.Errorf("VPS EQ v2 staging definition must be an experimental candidate Provider")
	}
	if len(definition.ImplementedSchemas) == 0 {
		return nil, fmt.Errorf("VPS EQ v2 staging definition implements no schemas")
	}
	seen := map[string]bool{}
	for _, schemaID := range definition.ImplementedSchemas {
		if seen[schemaID] || !containsString(definition.Descriptor.SupportedSchemas, schemaID) {
			return nil, fmt.Errorf("VPS EQ v2 staging schema %s is duplicate or not advertised", schemaID)
		}
		seen[schemaID] = true
	}
	// Build a private compiler definition with verified-shaped invariants. It
	// exists only in memory inside this adapter; the persisted definition and
	// returned descriptor continue to be candidate/non-routeable.
	private := EQV2ProviderDefinition{
		Descriptor:   definition.Descriptor,
		VPSID:        definition.VPSID,
		CredentialID: "staging:" + definition.StagingID,
		Binding:      definition.Binding,
	}
	private.Descriptor.Status = ProviderVerified
	private.Descriptor.Experimental = true
	private.Descriptor.SupportedSchemas = append([]string(nil), definition.ImplementedSchemas...)
	private.Binding.ConformedSchemas = append([]string(nil), definition.ImplementedSchemas...)
	compiler, err := NewVPSEQV2Adapter(private)
	if err != nil {
		return nil, fmt.Errorf("validate VPS EQ v2 staging compiler: %w", err)
	}
	return &VPSEQV2StagingAdapter{definition: definition, compiler: compiler, compilerID: private.CredentialID}, nil
}

func (a *VPSEQV2StagingAdapter) Descriptor() ProviderDescriptor {
	if a == nil {
		return ProviderDescriptor{}
	}
	return cloneProviderDescriptor(a.definition.Descriptor)
}

// Bind requires caller-selected staging metadata. This is the seam that stops
// a candidate adapter from participating in ordinary Catalog/SPAL resolution.
func (a *VPSEQV2StagingAdapter) Bind(instance ProviderInstance, instruction Instruction) (RuntimeBinding, error) {
	if a == nil || a.compiler == nil {
		return RuntimeBinding{}, fmt.Errorf("VPS EQ v2 staging adapter is nil")
	}
	if err := instruction.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if !containsString(a.definition.ImplementedSchemas, instruction.SchemaID) {
		return RuntimeBinding{}, fmt.Errorf("VPS EQ v2 staging implementation does not implement schema %s", instruction.SchemaID)
	}
	if err := instance.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	if instance.ProviderID != a.definition.Descriptor.ID || !strings.EqualFold(instance.Metadata["vpsforge_staging"], "true") ||
		!strings.EqualFold(instance.Metadata["vps_id"], a.definition.VPSID) {
		return RuntimeBinding{}, fmt.Errorf("project instance is not the explicitly selected VPS Forge staging target")
	}
	privateInstance := instance
	privateInstance.Status = InstanceVerified
	privateInstance.Metadata = cloneStringStringMap(instance.Metadata)
	privateInstance.Metadata["vps_id"] = a.definition.VPSID
	privateInstance.Metadata["vps_credential_id"] = a.compilerID
	privateInstance.Metadata["eq_v2_ready"] = "true"
	return a.compiler.Bind(privateInstance, instruction)
}

func (a *VPSEQV2StagingAdapter) Compile(instruction Instruction, binding RuntimeBinding) ([]PhysicalParameter, error) {
	if a == nil || a.compiler == nil {
		return nil, fmt.Errorf("VPS EQ v2 staging adapter is nil")
	}
	return a.compiler.Compile(instruction, binding)
}

func cloneEQV2StagingProviderDefinition(in EQV2StagingProviderDefinition) EQV2StagingProviderDefinition {
	out := in
	out.Descriptor = cloneProviderDescriptor(in.Descriptor)
	out.Binding = cloneEQV2ProviderDefinition(EQV2ProviderDefinition{Binding: in.Binding}).Binding
	out.ImplementedSchemas = append([]string(nil), in.ImplementedSchemas...)
	return out
}

func cloneProviderDescriptor(in ProviderDescriptor) ProviderDescriptor {
	out := in
	out.SupportedSchemas = append([]string(nil), in.SupportedSchemas...)
	out.ConformanceEvidence = append([]string(nil), in.ConformanceEvidence...)
	return out
}

func cloneStringStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+3)
	for key, value := range in {
		out[key] = value
	}
	return out
}
