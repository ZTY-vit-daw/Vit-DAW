package eqcontrolgraph

import (
	"fmt"
	"math"
	"strings"
)

type LiveSurface struct {
	Plugin     PluginIdentity
	Signature  string
	Parameters []LiveParameter
}

type LiveParameter struct {
	ID                string
	Name              string
	HostControllable  bool
	CurrentNormalized float64
	CurrentText       string
	CurrentPhysical   *float64
	Samples           []LiveNumericSample
	EnumValues        []LiveEnumValue
}

type LiveNumericSample struct {
	Normalized float64
	Physical   float64
}

type LiveEnumValue struct {
	Normalized float64
	Label      string
}

func ValidateLive(document Document, surface LiveSurface) (int, error) {
	if err := document.Validate(); err != nil {
		return 0, err
	}
	if !strings.EqualFold(strings.TrimSpace(document.Plugin.Name), strings.TrimSpace(surface.Plugin.Name)) {
		return 0, fmt.Errorf("VPS plugin name %q does not match live %q", document.Plugin.Name, surface.Plugin.Name)
	}
	if !strings.EqualFold(strings.TrimSpace(document.Plugin.Format), strings.TrimSpace(surface.Plugin.Format)) {
		return 0, fmt.Errorf("VPS format %q does not match live %q", document.Plugin.Format, surface.Plugin.Format)
	}
	if manufacturer := strings.TrimSpace(document.Plugin.Manufacturer); manufacturer != "" &&
		!strings.EqualFold(manufacturer, strings.TrimSpace(surface.Plugin.Manufacturer)) {
		return 0, fmt.Errorf("VPS manufacturer %q does not match live %q", document.Plugin.Manufacturer, surface.Plugin.Manufacturer)
	}
	if document.Plugin.Version != "" && document.Plugin.Version != strings.TrimSpace(surface.Plugin.Version) {
		return 0, fmt.Errorf("VPS version %q does not match live %q", document.Plugin.Version, surface.Plugin.Version)
	}
	if document.SurfaceSignature != surface.Signature {
		return 0, fmt.Errorf("VPS surface_signature %s does not match live %s", document.SurfaceSignature, surface.Signature)
	}
	index := make(map[string]LiveParameter, len(surface.Parameters))
	for _, parameter := range surface.Parameters {
		if _, exists := index[parameter.ID]; exists {
			return 0, fmt.Errorf("live surface repeats parameter_id %s", parameter.ID)
		}
		index[parameter.ID] = parameter
	}
	checks := 4
	for _, key := range sortedBindingKeys(document.Bindings) {
		binding := document.Bindings[key]
		parameter, ok := index[binding.ParameterID]
		if !ok {
			return checks, fmt.Errorf("binding %s parameter %s is absent from live surface", key, binding.ParameterID)
		}
		if !strings.EqualFold(strings.TrimSpace(binding.ParameterName), strings.TrimSpace(parameter.Name)) {
			return checks, fmt.Errorf("binding %s parameter name %q does not match live %q", key, binding.ParameterName, parameter.Name)
		}
		checks += 2
		if binding.Kind == "number" {
			for _, expected := range binding.Domain.Curve {
				matched := false
				for _, actual := range parameter.Samples {
					if math.Abs(actual.Normalized-expected[0]) > 1e-6 {
						continue
					}
					tolerance := math.Max(1e-6, math.Abs(expected[1])*1e-6)
					if math.Abs(actual.Physical-expected[1]) > tolerance {
						return checks, fmt.Errorf("binding %s curve point %g=%g does not match live %g", key, expected[0], expected[1], actual.Physical)
					}
					matched = true
					break
				}
				if !matched {
					return checks, fmt.Errorf("binding %s curve point %g was not freshly observed", key, expected[0])
				}
				checks++
			}
		} else {
			for semantic, expected := range binding.Values {
				matched := false
				for _, actual := range parameter.EnumValues {
					if math.Abs(actual.Normalized-expected.Normalized) <= 1e-6 && strings.TrimSpace(actual.Label) == strings.TrimSpace(expected.Label) {
						matched = true
						break
					}
				}
				if !matched {
					return checks, fmt.Errorf("binding %s enum %s=%g label %q was not freshly observed", key, semantic, expected.Normalized, expected.Label)
				}
				checks++
			}
		}
	}
	return checks, nil
}
