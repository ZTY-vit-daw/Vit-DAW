package plugingrabber

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"vit-daw-agent/internal/eqcontrolgraph"
)

// EQControlGraphRuntime is deliberately opaque to JSON/context serialization.
// It carries the validated data-only graph to the planner without publishing a
// second executable surface or embedding plug-in-specific behavior in Go.
type EQControlGraphRuntime struct {
	document   eqcontrolgraph.Document
	sectionKey string
}

func NewEQControlGraphRuntime(document eqcontrolgraph.Document, sectionKey string) (*EQControlGraphRuntime, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}
	if _, ok := document.Section(sectionKey); !ok {
		return nil, fmt.Errorf("control graph section %s is absent", sectionKey)
	}
	return &EQControlGraphRuntime{document: document, sectionKey: sectionKey}, nil
}

func (runtime *EQControlGraphRuntime) Compile(shape, action string, inputs map[string]interface{}) (eqcontrolgraph.CompileResult, error) {
	if runtime == nil {
		return eqcontrolgraph.CompileResult{}, fmt.Errorf("control graph runtime is nil")
	}
	return runtime.document.Compile(eqcontrolgraph.CompileRequest{
		SectionKey: runtime.sectionKey, Shape: shape, Action: action, Inputs: inputs,
	})
}

func BuildEQBandSummaryWithConfiguredControlGraph(digest ParameterDigest) (map[string]any, error) {
	surface := eqControlGraphLiveSurface(digest)
	allowCandidate := strings.TrimSpace(os.Getenv(eqcontrolgraph.AllowCandidateEnv)) == "1"
	resolved, _, err := eqcontrolgraph.ResolveDirectory(eqcontrolgraph.DefaultDirectory(), surface, allowCandidate)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return buildGenericEQBandSummary(digest), nil
	}
	return BuildEQBandSummaryWithControlGraph(digest, resolved)
}

func BuildEQBandSummaryWithControlGraph(digest ParameterDigest, resolved *eqcontrolgraph.ResolvedDocument) (map[string]any, error) {
	if resolved == nil {
		return nil, fmt.Errorf("resolved control graph is nil")
	}
	surface := eqControlGraphLiveSurface(digest)
	if _, err := eqcontrolgraph.ValidateLive(resolved.Document, surface); err != nil {
		return nil, err
	}
	parameterIndex := make(map[string]eqcontrolgraph.LiveParameter, len(surface.Parameters))
	for _, parameter := range surface.Parameters {
		parameterIndex[parameter.ID] = parameter
	}
	sections := make([]map[string]any, 0, len(resolved.Document.Sections))
	supportedSet := map[string]bool{}
	channels := map[string]bool{}
	capabilityFlags := map[string]bool{}
	for _, section := range resolved.Document.Sections {
		row, err := eqControlGraphSectionSummary(resolved.Document, section, parameterIndex)
		if err != nil {
			return nil, fmt.Errorf("section %s: %w", section.SectionKey, err)
		}
		sections = append(sections, row)
		for shape := range section.Shapes {
			supportedSet[shape] = true
			for _, action := range section.Shapes[shape].Actions {
				for _, input := range append(append([]string(nil), action.RequiredInputs...), action.OptionalInputs...) {
					capabilityFlags[input] = true
				}
			}
		}
		for _, bindingKey := range section.Bindings {
			channels[resolved.Document.Bindings[bindingKey].Channel] = true
		}
	}
	supported := make([]string, 0, len(supportedSet))
	for shape := range supportedSet {
		supported = append(supported, shape)
	}
	sort.Strings(supported)
	channelBindings := make([]string, 0, len(channels))
	for channel := range channels {
		channelBindings = append(channelBindings, channel)
	}
	sort.Strings(channelBindings)
	shapeCapabilities := aggregateControlGraphShapeCapabilities(resolved.Document.Sections, supported)
	hash := strings.TrimPrefix(resolved.Attestation.VPSHash, "sha256:")
	if len(hash) < 24 {
		return nil, fmt.Errorf("control graph attestation has an invalid vps_hash")
	}
	generation := "eqcg1_" + hash[:24]
	return map[string]any{
		"eq_model": resolved.Document.PublicClassification, "mapping_source": "vps_control_graph",
		"confidence":   1.0,
		"completeness": map[string]any{"complete": true, "complete_band_count": len(sections), "candidate_band_count": len(sections), "issues": []string{}},
		"capabilities": map[string]any{
			"frequency_adjustable": capabilityFlags["frequency_hz"], "fixed_frequency": resolved.Document.PublicClassification == "fixed_frequency",
			"gain": capabilityFlags["gain_db"], "q": capabilityFlags["q"], "shape": len(supported) > 1,
			"slope": capabilityFlags["slope_db_per_oct"], "filter_design": false,
			"activation": controlGraphHasAction(resolved.Document.Sections, "disable"), "multi_channel": len(channelBindings) > 1,
		},
		"set_eq_point_supported": controlGraphHasAction(resolved.Document.Sections, "upsert"),
		"channel_bindings":       channelBindings,
		"control_topology": map[string]any{
			"schema_version": "eq-control-topology/v1", "generation": generation,
			"public_classification": resolved.Document.PublicClassification,
			"addressing_kinds":      controlGraphAddressingKinds(resolved.Document.Sections),
			"shape_capabilities":    shapeCapabilities,
		},
		"band_count": 0, "bands": []map[string]any{}, "section_count": len(sections), "sections": sections,
		"supported_filter_kinds": supported,
		"how_to_pick_a_band":     "Select a VPS control-graph section whose action contract accepts every explicit input.",
		"vps_control_graph": map[string]any{
			"schema_version": resolved.Document.SchemaVersion, "plugin": resolved.Document.Plugin.Name,
			"surface_signature": resolved.Document.SurfaceSignature, "verification_status": resolved.Attestation.Status,
			"unresolved": resolved.Document.Unresolved,
		},
	}, nil
}

func eqControlGraphSectionSummary(document eqcontrolgraph.Document, section eqcontrolgraph.Section,
	parameters map[string]eqcontrolgraph.LiveParameter) (map[string]any, error) {
	row := map[string]any{
		"section": section.SectionKey, "primary": true, "complete": true, "issues": []string{},
		"active":              section.Activation.State == "always_active",
		"activation_strategy": section.Activation.State,
		"activation":          map[string]any{"strategy": section.Activation.State, "known": section.Activation.State == "always_active", "active": section.Activation.State == "always_active"},
		"addressing":          section.Addressing, "deallocatable": section.Addressing == "allocatable",
		"exclusion_codes": []string{}, "control_graph_runtime": &EQControlGraphRuntime{document: document, sectionKey: section.SectionKey},
	}
	reachable := make([]string, 0, len(section.Shapes))
	capabilities := make([]map[string]any, 0, len(section.Shapes))
	for shape, definition := range section.Shapes {
		reachable = append(reachable, shape)
		actions := map[string]any{"upsert": false, "modify": false, "disable": false, "remove": false, "undo": false}
		for action := range definition.Actions {
			actions[action] = true
		}
		if actions["upsert"].(bool) || actions["modify"].(bool) {
			actions["undo"] = true
		}
		capabilities = append(capabilities, map[string]any{"shape": shape, "actions": actions, "rejection_codes": map[string]any{}})
	}
	sort.Strings(reachable)
	sort.Slice(capabilities, func(i, j int) bool {
		return fmt.Sprint(capabilities[i]["shape"]) < fmt.Sprint(capabilities[j]["shape"])
	})
	row["reachable_kinds"] = reachable
	row["shape_capabilities"] = capabilities
	channels := map[string]bool{}
	for _, bindingKey := range section.Bindings {
		channels[document.Bindings[bindingKey].Channel] = true
	}
	channelBindings := make([]string, 0, len(channels))
	for channel := range channels {
		channelBindings = append(channelBindings, channel)
	}
	sort.Strings(channelBindings)
	row["channel_bindings"] = channelBindings
	if section.Selection.FrequencyBinding != "" {
		bindingKey := section.Bindings[section.Selection.FrequencyBinding]
		binding := document.Bindings[bindingKey]
		parameter, ok := parameters[binding.ParameterID]
		if !ok {
			return nil, fmt.Errorf("selection parameter %s is absent", binding.ParameterID)
		}
		row["frequency_bindings"] = []map[string]any{controlGraphNumericBindingRow(binding, parameter)}
	} else if section.Selection.FixedFrequencyHz != nil {
		row["fixed_freq_hz"] = *section.Selection.FixedFrequencyHz
	}
	return row, nil
}

func controlGraphNumericBindingRow(binding eqcontrolgraph.Binding, parameter eqcontrolgraph.LiveParameter) map[string]any {
	domain := binding.Domain
	row := map[string]any{
		"param_id": binding.ParameterID, "name": binding.ParameterName, "channel": binding.Channel,
		"current_normalized": parameter.CurrentNormalized, "current_text": parameter.CurrentText,
		"domain": map[string]any{"unit": domain.Unit, "min": domain.Minimum, "max": domain.Maximum, "scale": controlGraphScale(domain.ValueLaw)},
		"curve":  append([][2]float64(nil), domain.Curve...),
	}
	if parameter.CurrentPhysical != nil {
		row["current_physical"] = *parameter.CurrentPhysical
	}
	return row
}

func controlGraphScale(valueLaw string) string {
	if strings.EqualFold(valueLaw, "logarithmic") {
		return "log"
	}
	return "linear"
}

func eqControlGraphLiveSurface(digest ParameterDigest) eqcontrolgraph.LiveSurface {
	identity := eqcontrolgraph.PluginIdentity{
		Name:         digest.PluginName,
		Manufacturer: strings.TrimSpace(fmt.Sprint(digest.PluginIdentity["manufacturer"])),
		Format:       strings.TrimSpace(fmt.Sprint(digest.PluginIdentity["plugin_format"])),
		Version:      strings.TrimSpace(fmt.Sprint(digest.PluginIdentity["version"])),
	}
	if identity.Format == "" || identity.Format == "<nil>" {
		identity.Format = "VST3"
	}
	if identity.Manufacturer == "<nil>" {
		identity.Manufacturer = ""
	}
	if identity.Version == "<nil>" {
		identity.Version = ""
	}
	surface := eqcontrolgraph.LiveSurface{Plugin: identity, Signature: digest.CurrentParamSignatureHash}
	for _, parameter := range digest.Parameters {
		normalized, _ := controlGraphNumericValue(parameter.NormalizedValue)
		unit := strings.TrimSpace(parameter.Unit)
		if unit == "" && parameter.DisplayDomainCandidate != nil {
			unit = strings.TrimSpace(parameter.DisplayDomainCandidate.Unit)
		}
		live := eqcontrolgraph.LiveParameter{ID: parameter.ID, Name: parameter.Name,
			HostControllable: parameter.HostControllable, CurrentNormalized: normalized, CurrentText: parameter.ValueText}
		if value, ok := controlGraphDisplayNumber(parameter.ValueText, unit); ok {
			live.CurrentPhysical = &value
		}
		if parameter.DisplayProbe != nil {
			for _, sample := range parameter.DisplayProbe.Samples {
				if physical, ok := controlGraphDisplayNumber(sample.Text, unit); ok {
					live.Samples = append(live.Samples, eqcontrolgraph.LiveNumericSample{Normalized: sample.NormalizedValue, Physical: physical})
				}
			}
			for _, value := range parameter.DisplayProbe.DiscreteLabels {
				if normalized, ok := controlGraphNumericValue(value.Value); ok {
					live.EnumValues = append(live.EnumValues, eqcontrolgraph.LiveEnumValue{Normalized: normalized, Label: value.Label})
				}
			}
		}
		surface.Parameters = append(surface.Parameters, live)
	}
	return surface
}

func controlGraphDisplayNumber(text, unit string) (float64, bool) {
	if strings.EqualFold(strings.TrimSpace(unit), "hz") {
		value, ok := ParseEQFrequencyText(text)
		return value, ok && !math.IsNaN(value) && !math.IsInf(value, 0)
	}
	if value, ok := ParseEQLocalizedNumber(text); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
		return value, true
	}
	return 0, false
}

func controlGraphNumericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func aggregateControlGraphShapeCapabilities(sections []eqcontrolgraph.Section, shapes []string) []map[string]any {
	rows := make([]map[string]any, 0, len(shapes))
	for _, shape := range shapes {
		actions := map[string]any{"upsert": false, "modify": false, "disable": false, "remove": false, "undo": false}
		for _, section := range sections {
			definition, ok := section.Shapes[shape]
			if !ok {
				continue
			}
			for action := range definition.Actions {
				actions[action] = true
			}
		}
		if actions["upsert"].(bool) || actions["modify"].(bool) {
			actions["undo"] = true
		}
		rows = append(rows, map[string]any{"shape": shape, "actions": actions, "rejection_codes": map[string]any{}})
	}
	return rows
}

func controlGraphHasAction(sections []eqcontrolgraph.Section, wanted string) bool {
	for _, section := range sections {
		for _, shape := range section.Shapes {
			if _, ok := shape.Actions[wanted]; ok {
				return true
			}
		}
	}
	return false
}

func controlGraphAddressingKinds(sections []eqcontrolgraph.Section) []string {
	set := map[string]bool{}
	for _, section := range sections {
		set[section.Addressing] = true
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
