package spal

import (
	"fmt"
	"math"
	"strings"
)

// StaticEQProviderDefinition is the small, credential-bound adapter contract
// used by VPS v3 at runtime. It deliberately contains only the conformed
// control surface required by spectral.static_eq.v0; project/track/plugin
// identity remains on ProviderInstance.
type StaticEQProviderDefinition struct {
	Descriptor            ProviderDescriptor          `json:"descriptor"`
	VPSID                 string                      `json:"vps_id"`
	CredentialID          string                      `json:"credential_id"`
	ComponentID           string                      `json:"component_id"`
	FilterTypeParameterID string                      `json:"filter_type_parameter_id"`
	ParameterBindings     map[string]ParameterBinding `json:"parameter_bindings"`
}

// VPSStaticEQAdapter is a Credential-derived SPAL adapter. It has no fallback
// knowledge of a plug-in vendor or parameter IDs: all mappings originate from
// a verified VPS document and are validated again before the adapter is made
// available to a project instance.
type VPSStaticEQAdapter struct {
	definition StaticEQProviderDefinition
}

// NewVPSStaticEQAdapter creates a static Bell adapter from a verified VPS v3
// Credential definition. A malformed or incomplete mapping is rejected here,
// before it can enter a SPAL Registry.
func NewVPSStaticEQAdapter(definition StaticEQProviderDefinition) (*VPSStaticEQAdapter, error) {
	if err := definition.valid(); err != nil {
		return nil, err
	}
	return &VPSStaticEQAdapter{definition: cloneStaticEQProviderDefinition(definition)}, nil
}

func (a *VPSStaticEQAdapter) Descriptor() ProviderDescriptor {
	if a == nil {
		return ProviderDescriptor{}
	}
	descriptor := a.definition.Descriptor
	descriptor.SupportedSchemas = append([]string(nil), descriptor.SupportedSchemas...)
	descriptor.ConformanceEvidence = append([]string(nil), descriptor.ConformanceEvidence...)
	return descriptor
}

func (a *VPSStaticEQAdapter) Bind(instance ProviderInstance, instruction Instruction) (RuntimeBinding, error) {
	if a == nil {
		return RuntimeBinding{}, fmt.Errorf("VPS static EQ adapter is nil")
	}
	if err := instruction.Validate(); err != nil {
		return RuntimeBinding{}, err
	}
	if instruction.SchemaID != StaticBellControlID {
		return RuntimeBinding{}, fmt.Errorf("VPS static EQ adapter does not implement %s", instruction.SchemaID)
	}
	if err := instance.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	if instance.ProviderID != a.definition.Descriptor.ID || instance.Status != InstanceVerified {
		return RuntimeBinding{}, fmt.Errorf("project Provider Instance is not verified for the selected VPS Credential")
	}
	if !strings.EqualFold(strings.TrimSpace(instance.Metadata["vps_credential_id"]), a.definition.CredentialID) ||
		!strings.EqualFold(strings.TrimSpace(instance.Metadata["vps_id"]), a.definition.VPSID) ||
		!strings.EqualFold(strings.TrimSpace(instance.Metadata["static_bell_ready"]), "true") {
		return RuntimeBinding{}, fmt.Errorf("project Provider Instance no longer matches the verified VPS static-Bell Credential")
	}
	binding := RuntimeBinding{
		ID:                "binding:" + instance.ID + ":" + StaticBellControlID,
		SchemaID:          StaticBellControlID,
		Provider:          a.Descriptor(),
		Instance:          instance,
		ParameterBindings: cloneParameterBindings(a.definition.ParameterBindings),
		Invariants: map[string]string{
			"vps_id":               a.definition.VPSID,
			"vps_credential_id":    a.definition.CredentialID,
			"component_id":         a.definition.ComponentID,
			"filter_type_param_id": a.definition.FilterTypeParameterID,
			"static_bell_ready":    "true",
		},
	}
	if err := binding.Valid(); err != nil {
		return RuntimeBinding{}, err
	}
	return binding, nil
}

func (a *VPSStaticEQAdapter) Compile(instruction Instruction, binding RuntimeBinding) ([]PhysicalParameter, error) {
	if a == nil {
		return nil, fmt.Errorf("VPS static EQ adapter is nil")
	}
	if err := binding.Valid(); err != nil {
		return nil, err
	}
	if binding.Provider.ID != a.definition.Descriptor.ID || binding.SchemaID != StaticBellControlID ||
		!strings.EqualFold(binding.Invariants["vps_credential_id"], a.definition.CredentialID) ||
		!strings.EqualFold(binding.Invariants["vps_id"], a.definition.VPSID) ||
		!strings.EqualFold(binding.Invariants["static_bell_ready"], "true") {
		return nil, fmt.Errorf("binding is not for the selected verified VPS static-Bell Credential")
	}
	values, err := instruction.StaticBellValues()
	if err != nil {
		return nil, err
	}
	for key, expected := range a.definition.ParameterBindings {
		actual, ok := binding.ParameterBindings[key]
		if !ok || actual != expected {
			return nil, fmt.Errorf("binding parameter %s does not match the verified VPS mapping", key)
		}
	}
	rows := make([]PhysicalParameter, 0, 4)
	for _, item := range []struct {
		key   string
		value float64
	}{
		{"enabled", 1},
		{"center_frequency_hz", values.CenterFrequencyHz},
		{"gain_db", values.GainDB},
		{"q", values.Q},
	} {
		parameter, ok := binding.ParameterBindings[item.key]
		if !ok {
			return nil, fmt.Errorf("binding omits %s", item.key)
		}
		normalized, err := staticEQNormalizedValue(parameter, item.value)
		if err != nil {
			return nil, fmt.Errorf("VPS static EQ %s: %w", item.key, err)
		}
		rows = append(rows, PhysicalParameter{ParameterID: parameter.ParameterID, Value: normalized, Unit: parameter.Unit})
	}
	return rows, nil
}

func (d StaticEQProviderDefinition) valid() error {
	if strings.TrimSpace(d.VPSID) == "" || strings.TrimSpace(d.CredentialID) == "" || strings.TrimSpace(d.ComponentID) == "" || strings.TrimSpace(d.FilterTypeParameterID) == "" {
		return fmt.Errorf("VPS static EQ definition requires VPS, Credential, component and filter-type binding")
	}
	if err := d.Descriptor.Valid(); err != nil {
		return err
	}
	if d.Descriptor.Status != ProviderVerified || !supports(d.Descriptor, StaticBellControlID) {
		return fmt.Errorf("VPS static EQ definition must expose a verified %s Provider", StaticBellControlID)
	}
	for _, key := range []string{"enabled", "center_frequency_hz", "gain_db", "q"} {
		binding, ok := d.ParameterBindings[key]
		if !ok {
			return fmt.Errorf("VPS static EQ definition omits %s", key)
		}
		if err := validStaticEQParameterBinding(key, binding); err != nil {
			return err
		}
	}
	return nil
}

func validStaticEQParameterBinding(key string, binding ParameterBinding) error {
	if strings.TrimSpace(binding.ParameterID) == "" || !finite(binding.Min) || !finite(binding.Max) || binding.Max <= binding.Min {
		return fmt.Errorf("VPS static EQ %s requires a finite, non-empty bounded parameter mapping", key)
	}
	scale := strings.ToLower(strings.TrimSpace(binding.Scale))
	switch scale {
	case "linear", "log", "logarithmic":
	case "":
		return fmt.Errorf("VPS static EQ %s mapping has no confirmed display scale", key)
	default:
		return fmt.Errorf("VPS static EQ %s mapping has unsupported display scale %q", key, binding.Scale)
	}
	if (scale == "log" || scale == "logarithmic") && binding.Min <= 0 {
		return fmt.Errorf("VPS static EQ %s logarithmic mapping must have a positive minimum", key)
	}
	return nil
}

func staticEQNormalizedValue(binding ParameterBinding, value float64) (float64, error) {
	if !finite(value) || !finite(binding.Min) || !finite(binding.Max) || binding.Max <= binding.Min {
		return 0, fmt.Errorf("invalid semantic value or display range")
	}
	if value < binding.Min || value > binding.Max {
		return 0, fmt.Errorf("%.6f is outside conformed display range %.6f..%.6f", value, binding.Min, binding.Max)
	}
	if value == binding.Min {
		return 0, nil
	}
	if value == binding.Max {
		return 1, nil
	}
	switch strings.ToLower(strings.TrimSpace(binding.Scale)) {
	case "linear":
		return (value - binding.Min) / (binding.Max - binding.Min), nil
	case "log", "logarithmic":
		if binding.Min <= 0 || value <= 0 {
			return 0, fmt.Errorf("logarithmic mapping requires positive values")
		}
		return math.Log(value/binding.Min) / math.Log(binding.Max/binding.Min), nil
	default:
		return 0, fmt.Errorf("unsupported display scale %q", binding.Scale)
	}
}

func cloneStaticEQProviderDefinition(input StaticEQProviderDefinition) StaticEQProviderDefinition {
	output := input
	output.Descriptor.SupportedSchemas = append([]string(nil), input.Descriptor.SupportedSchemas...)
	output.Descriptor.ConformanceEvidence = append([]string(nil), input.Descriptor.ConformanceEvidence...)
	output.ParameterBindings = cloneParameterBindings(input.ParameterBindings)
	return output
}

func cloneParameterBindings(input map[string]ParameterBinding) map[string]ParameterBinding {
	output := make(map[string]ParameterBinding, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
