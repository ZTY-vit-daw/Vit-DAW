package spal

import (
	"context"
	"fmt"
)

// PreimageReader is a read-only SPAL service. It captures the exact physical
// values that must be restored if a later VSP batch partially fails. Capability
// code never receives raw parameter IDs through this interface.
type PreimageReader interface {
	CaptureSPALPreimage(context.Context, RuntimeBinding, []PhysicalParameter) ([]PhysicalParameter, error)
}

type Runtime struct {
	Registry *Registry
}

type Preparation struct {
	Resolution Resolution         `json:"resolution"`
	Manifest   *ExecutionManifest `json:"manifest,omitempty"`
}

// Prepare is the control-plane handoff from a capability's semantic parameter
// instruction to a frozen-ready execution manifest. It makes no mutation.
func (r Runtime) Prepare(ctx context.Context, instruction Instruction, instances []ProviderInstance, options ResolveOptions, reader PreimageReader) (Preparation, error) {
	if r.Registry == nil {
		return Preparation{}, fmt.Errorf("SPAL registry is required")
	}
	resolution, err := r.Registry.Resolve(instruction, instances, options)
	if err != nil {
		return Preparation{}, err
	}
	prepared := Preparation{Resolution: resolution}
	if resolution.Status != ResolutionBound || resolution.Binding == nil {
		return prepared, nil
	}
	if reader == nil {
		return Preparation{}, fmt.Errorf("SPAL preimage reader is required for a bound control")
	}
	adapter, ok := r.Registry.adapter(resolution.Binding.Provider.ID)
	if !ok {
		return Preparation{}, fmt.Errorf("bound SPAL provider %s is no longer registered", resolution.Binding.Provider.ID)
	}
	writes, err := adapter.Compile(instruction, *resolution.Binding)
	if err != nil {
		return Preparation{}, err
	}
	preimage, err := reader.CaptureSPALPreimage(ctx, *resolution.Binding, writes)
	if err != nil {
		return Preparation{}, fmt.Errorf("capture SPAL preimage: %w", err)
	}
	manifest, err := CompileManifest(resolution, instruction, preimage, adapter)
	if err != nil {
		return Preparation{}, err
	}
	prepared.Manifest = &manifest
	return prepared, nil
}
