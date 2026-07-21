package spal

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry owns only verified adapters. A Plugin Learning result remains a
// candidate outside this registry until it has conformance evidence.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

func (r *Registry) Register(adapter Adapter) error {
	if r == nil || adapter == nil {
		return fmt.Errorf("SPAL registry and adapter are required")
	}
	descriptor := adapter.Descriptor()
	if err := descriptor.Valid(); err != nil {
		return err
	}
	if descriptor.Status != ProviderVerified {
		return fmt.Errorf("provider %s is not verified", descriptor.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[descriptor.ID]; exists {
		return fmt.Errorf("provider %s is already registered", descriptor.ID)
	}
	r.adapters[descriptor.ID] = adapter
	return nil
}

func (r *Registry) Providers() []ProviderDescriptor {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderDescriptor, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		d := adapter.Descriptor()
		d.SupportedSchemas = sortedCopy(d.SupportedSchemas)
		d.ConformanceEvidence = append([]string(nil), d.ConformanceEvidence...)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Catalog(targetRef string, instances []ProviderInstance) []ControlOffer {
	if strings.TrimSpace(targetRef) == "" {
		return nil
	}
	schemas := []string{StaticBellControlID, EQBandPatchControlID, EQPassFilterPatchControlID, EQDynamicBandPatchControlID, EQOutputPatchControlID}
	offers := make([]ControlOffer, 0, len(schemas))
	for _, schemaID := range schemas {
		resolution, err := r.Resolve(catalogProbeInstruction(schemaID, targetRef), instances, ResolveOptions{})
		offer := ControlOffer{SchemaID: schemaID, TargetRef: targetRef, Status: ResolutionNoVerifiedProvider}
		if schema, ok := Schema(schemaID); ok {
			offer.Parameters = append([]ParameterSpec(nil), schema.Parameters...)
		}
		if err != nil {
			offer.Reason = err.Error()
		} else {
			offer.Status, offer.Reason = resolution.Status, resolution.Reason
		}
		offers = append(offers, offer)
	}
	return offers
}

func catalogProbeInstruction(schemaID, targetRef string) Instruction {
	instruction := Instruction{SchemaID: schemaID, TargetRef: targetRef, Parameters: map[string]float64{}, StringParameters: map[string]string{}}
	switch schemaID {
	case StaticBellControlID:
		instruction.Parameters = map[string]float64{"center_frequency_hz": 100, "gain_db": -1, "q": 1}
	case EQBandPatchControlID:
		instruction.Parameters = map[string]float64{"enabled": 1, "frequency_hz": 100, "gain_db": -1, "q": 1}
		instruction.StringParameters = map[string]string{"band_ref": "b1", "response_shape": "bell"}
	case EQPassFilterPatchControlID:
		instruction.Parameters = map[string]float64{"enabled": 1, "cutoff_frequency_hz": 80, "slope_db_per_octave": 12}
		instruction.StringParameters = map[string]string{"filter_kind": "highpass"}
	case EQDynamicBandPatchControlID:
		instruction.Parameters = map[string]float64{"threshold_db": -20, "ratio": 2, "attack_ms": 10, "release_ms": 100}
		instruction.StringParameters = map[string]string{"band_ref": "b2", "dynamics_mode": "normal", "routing_scope": "independent"}
	case EQOutputPatchControlID:
		instruction.Parameters = map[string]float64{"output_gain_db": 0}
	}
	return instruction
}

// Resolve has no automatic reference-plugin fallback. A registered Adapter is
// usable only when the current project supplies a matching verified instance.
func (r *Registry) Resolve(instruction Instruction, instances []ProviderInstance, options ResolveOptions) (Resolution, error) {
	if r == nil {
		return Resolution{}, fmt.Errorf("SPAL registry is nil")
	}
	if err := instruction.Validate(); err != nil {
		return Resolution{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	matchingProvider := false
	providerIDs := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)
	for _, providerID := range providerIDs {
		adapter := r.adapters[providerID]
		descriptor := adapter.Descriptor()
		if !supports(descriptor, instruction.SchemaID) {
			continue
		}
		if requested := strings.TrimSpace(options.RequestedProviderID); requested != "" && requested != descriptor.ID {
			continue
		}
		matchingProvider = true
		for _, instance := range instances {
			if instance.ProviderID != descriptor.ID || instance.TargetRef != instruction.TargetRef || instance.Status != InstanceVerified {
				continue
			}
			binding, err := adapter.Bind(instance, instruction)
			if err != nil {
				continue
			}
			return Resolution{Status: ResolutionBound, Binding: &binding}, nil
		}
		if options.AllowProvisioning && strings.TrimSpace(options.RequestedProviderID) == descriptor.ID {
			return Resolution{
				Status: ResolutionRequiresProvisioning,
				Reason: "a verified Provider was explicitly selected but has no bound instance; create a confirmed provisioning proposal",
			}, nil
		}
	}
	if matchingProvider {
		return Resolution{
			Status:              ResolutionNoVerifiedProvider,
			Reason:              "no verified Provider instance is bound to this target; generate and conform a Plugin Skill before execution",
			ProviderRequired:    true,
			RequiredPluginSkill: true,
		}, nil
	}
	return Resolution{
		Status:              ResolutionNoVerifiedProvider,
		Reason:              "no verified Provider implements this semantic control; generate and conform a Plugin Skill",
		ProviderRequired:    true,
		RequiredPluginSkill: true,
	}, nil
}

func supports(descriptor ProviderDescriptor, schemaID string) bool {
	for _, supported := range descriptor.SupportedSchemas {
		if strings.TrimSpace(supported) == strings.TrimSpace(schemaID) {
			return true
		}
	}
	return false
}

func (r *Registry) adapter(providerID string) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[providerID]
	return adapter, ok
}
