package eqcontrolgraph

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

var publicInputs = map[string]valueType{
	"frequency_hz":     typeNumber,
	"gain_db":          typeNumber,
	"q":                typeNumber,
	"slope_db_per_oct": typeNumber,
}

var publicShapes = map[string]bool{
	"bell": true, "low_shelf": true, "high_shelf": true, "low_cut": true, "high_cut": true,
}

var forwardActions = map[string]bool{"upsert": true, "modify": true, "disable": true, "remove": true}

const (
	maxBindingsPerDocument = 4096
	maxSectionsPerDocument = 512
	maxCurvePoints         = 4096
	maxEnumValues          = 2048
	maxAliasesPerSection   = 512
	maxProgramsPerSection  = 64
	maxInputsPerProgram    = 16
)

func (document Document) Validate() error {
	if document.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %s", SchemaVersion)
	}
	if strings.TrimSpace(document.Plugin.Name) == "" || !strings.EqualFold(strings.TrimSpace(document.Plugin.Format), "VST3") {
		return fmt.Errorf("plugin name and VST3 format are required")
	}
	if strings.TrimSpace(document.SurfaceSignature) == "" {
		return fmt.Errorf("surface_signature is required")
	}
	switch document.PublicClassification {
	case "fixed_slot_adjustable", "free_floating", "fixed_frequency":
	default:
		return fmt.Errorf("unsupported public_classification %q", document.PublicClassification)
	}
	switch document.ChannelContract {
	case "shared", "mirrored", "mixed":
	default:
		return fmt.Errorf("unsupported channel_contract %q", document.ChannelContract)
	}
	if len(document.Bindings) == 0 || len(document.Sections) == 0 {
		return fmt.Errorf("bindings and sections are required")
	}
	if len(document.Bindings) > maxBindingsPerDocument || len(document.Sections) > maxSectionsPerDocument {
		return fmt.Errorf("document exceeds binding/section resource limits")
	}
	parameterIDs := map[string]string{}
	for _, key := range sortedBindingKeys(document.Bindings) {
		binding := document.Bindings[key]
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("binding key is empty")
		}
		if err := validateBinding(key, binding); err != nil {
			return err
		}
		if previous := parameterIDs[binding.ParameterID]; previous != "" {
			return fmt.Errorf("bindings %s and %s reuse parameter_id %s", previous, key, binding.ParameterID)
		}
		parameterIDs[binding.ParameterID] = key
	}
	sections := map[string]bool{}
	for index := range document.Sections {
		section := document.Sections[index]
		if strings.TrimSpace(section.SectionKey) == "" || sections[section.SectionKey] {
			return fmt.Errorf("section %d has empty or duplicate section_key", index)
		}
		sections[section.SectionKey] = true
		if err := validateSection(section, document.Bindings); err != nil {
			return fmt.Errorf("section %s: %w", section.SectionKey, err)
		}
	}
	return nil
}

func validateBinding(key string, binding Binding) error {
	if strings.TrimSpace(binding.ParameterID) == "" || strings.TrimSpace(binding.ParameterName) == "" {
		return fmt.Errorf("binding %s requires parameter_id and parameter_name", key)
	}
	switch binding.Channel {
	case "shared", "left", "right":
	default:
		return fmt.Errorf("binding %s has invalid channel %q", key, binding.Channel)
	}
	switch binding.Kind {
	case "number":
		if binding.Domain == nil || len(binding.Values) != 0 {
			return fmt.Errorf("numeric binding %s requires domain and forbids enum values", key)
		}
		if err := validateNumericDomain(*binding.Domain); err != nil {
			return fmt.Errorf("binding %s: %w", key, err)
		}
	case "enum":
		if binding.Domain != nil || len(binding.Values) == 0 {
			return fmt.Errorf("enum binding %s requires values and forbids numeric domain", key)
		}
		if len(binding.Values) > maxEnumValues {
			return fmt.Errorf("enum binding %s exceeds value resource limit", key)
		}
		normalized := map[float64]string{}
		labels := map[string]string{}
		for semantic, value := range binding.Values {
			if strings.TrimSpace(semantic) == "" || strings.TrimSpace(value.Label) == "" || value.Normalized < 0 || value.Normalized > 1 || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
				return fmt.Errorf("enum binding %s has invalid value %q", key, semantic)
			}
			if previous := normalized[value.Normalized]; previous != "" {
				return fmt.Errorf("enum binding %s values %s and %s share normalized %g", key, previous, semantic, value.Normalized)
			}
			normalized[value.Normalized] = semantic
			labelKey := strings.ToLower(strings.TrimSpace(value.Label))
			if previous := labels[labelKey]; previous != "" {
				return fmt.Errorf("enum binding %s values %s and %s share label %q", key, previous, semantic, value.Label)
			}
			labels[labelKey] = semantic
		}
	default:
		return fmt.Errorf("binding %s has unsupported kind %q", key, binding.Kind)
	}
	return nil
}

func validateNumericDomain(domain NumericDomain) error {
	if domain.Minimum > domain.Maximum || math.IsNaN(domain.Minimum) || math.IsNaN(domain.Maximum) || math.IsInf(domain.Minimum, 0) || math.IsInf(domain.Maximum, 0) {
		return fmt.Errorf("numeric domain has invalid range")
	}
	if len(domain.Curve) < 2 {
		return fmt.Errorf("numeric domain requires at least two observed curve points")
	}
	if len(domain.Curve) > maxCurvePoints {
		return fmt.Errorf("numeric domain exceeds curve resource limit")
	}
	direction := 0
	for index, point := range domain.Curve {
		if point[0] < 0 || point[0] > 1 || math.IsNaN(point[0]) || math.IsNaN(point[1]) || math.IsInf(point[0], 0) || math.IsInf(point[1], 0) {
			return fmt.Errorf("curve point %d is invalid", index)
		}
		if point[1] < domain.Minimum-1e-9 || point[1] > domain.Maximum+1e-9 {
			return fmt.Errorf("curve point %d physical value %g is outside [%g,%g]", index, point[1], domain.Minimum, domain.Maximum)
		}
		if index == 0 {
			continue
		}
		previous := domain.Curve[index-1]
		if point[0] <= previous[0] {
			return fmt.Errorf("curve normalized values are not strictly increasing")
		}
		delta := point[1] - previous[1]
		if math.Abs(delta) <= 1e-12 {
			return fmt.Errorf("curve physical values are not strictly monotonic")
		}
		currentDirection := 1
		if delta < 0 {
			currentDirection = -1
		}
		if direction == 0 {
			direction = currentDirection
		} else if direction != currentDirection {
			return fmt.Errorf("curve physical values are not monotonic")
		}
	}
	return nil
}

func validateSection(section Section, bindings map[string]Binding) error {
	switch section.Addressing {
	case "resident", "allocatable":
	default:
		return fmt.Errorf("invalid addressing %q", section.Addressing)
	}
	switch section.Activation.State {
	case "always_active", "explicit", "sentinel", "allocatable":
	default:
		return fmt.Errorf("invalid activation state %q", section.Activation.State)
	}
	if len(section.Bindings) == 0 || len(section.Shapes) == 0 || len(section.Programs) == 0 {
		return fmt.Errorf("bindings, shapes, and programs are required")
	}
	if len(section.Bindings) > maxAliasesPerSection || len(section.Programs) > maxProgramsPerSection {
		return fmt.Errorf("section exceeds alias/program resource limits")
	}
	for alias, key := range section.Bindings {
		if strings.TrimSpace(alias) == "" {
			return fmt.Errorf("binding alias is empty")
		}
		if _, ok := bindings[key]; !ok {
			return fmt.Errorf("binding alias %s references unknown binding %s", alias, key)
		}
	}
	if section.Selection.FrequencyBinding != "" {
		key, ok := section.Bindings[section.Selection.FrequencyBinding]
		if !ok || bindings[key].Kind != "number" {
			return fmt.Errorf("selection frequency_binding %q is not a numeric local binding", section.Selection.FrequencyBinding)
		}
	}
	if section.Selection.FrequencyBinding == "" && section.Selection.FixedFrequencyHz == nil {
		return fmt.Errorf("selection requires frequency_binding or fixed_frequency_hz")
	}
	for name, program := range section.Programs {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("program name is empty")
		}
		if err := validateProgram(name, program, section, bindings); err != nil {
			return err
		}
	}
	for shapeName, shape := range section.Shapes {
		if !publicShapes[shapeName] || len(shape.Actions) == 0 {
			return fmt.Errorf("shape %q is unsupported or has no actions", shapeName)
		}
		for action, contract := range shape.Actions {
			if !forwardActions[action] {
				return fmt.Errorf("shape %s has unsupported action %q", shapeName, action)
			}
			program, ok := section.Programs[contract.Program]
			if !ok {
				return fmt.Errorf("shape %s action %s references unknown program %s", shapeName, action, contract.Program)
			}
			if err := validateActionContract(shapeName, action, contract, program); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProgram(name string, program Program, section Section, bindings map[string]Binding) error {
	if len(program.Writes) == 0 {
		return fmt.Errorf("program %s has no writes", name)
	}
	inputTypes := map[string]valueType{"shape": typeString, "action": typeString}
	if len(program.Inputs) > maxInputsPerProgram {
		return fmt.Errorf("program %s exceeds input resource limit", name)
	}
	for input, spec := range program.Inputs {
		kind, ok := parseValueType(spec.Type)
		if !ok || strings.TrimSpace(input) == "" || input == "shape" || input == "action" {
			return fmt.Errorf("program %s has invalid input %q type %q", name, input, spec.Type)
		}
		inputTypes[input] = kind
	}
	if len(program.Writes) > 128 {
		return fmt.Errorf("program %s exceeds maximum write count 128", name)
	}
	for index, write := range program.Writes {
		bindingKey, ok := section.Bindings[write.Binding]
		if !ok {
			return fmt.Errorf("program %s write %d references unknown local binding %s", name, index, write.Binding)
		}
		binding := bindings[bindingKey]
		if strings.TrimSpace(write.Role) == "" {
			return fmt.Errorf("program %s write %d has empty role", name, index)
		}
		if write.Phase != "" && write.Phase != "activation" {
			return fmt.Errorf("program %s write %d has invalid phase %q", name, index, write.Phase)
		}
		if write.When != nil {
			nodes := 0
			kind, err := inferExpressionType(*write.When, inputTypes, 0, &nodes)
			if err != nil {
				return fmt.Errorf("program %s write %d condition: %w", name, index, err)
			}
			if kind != typeBoolean {
				return fmt.Errorf("program %s write %d condition is %s, want boolean", name, index, kind)
			}
		}
		nodes := 0
		kind, err := inferExpressionType(write.Value, inputTypes, 0, &nodes)
		if err != nil {
			return fmt.Errorf("program %s write %d value: %w", name, index, err)
		}
		expected := typeNumber
		if binding.Kind == "enum" {
			expected = typeString
		}
		if kind != expected {
			return fmt.Errorf("program %s write %d value is %s, binding %s requires %s", name, index, kind, bindingKey, expected)
		}
	}
	return nil
}

func validateActionContract(shape, action string, contract ActionContract, program Program) error {
	seen := map[string]bool{}
	for _, input := range append(append([]string(nil), contract.RequiredInputs...), contract.OptionalInputs...) {
		if seen[input] {
			return fmt.Errorf("shape %s action %s repeats input %s", shape, action, input)
		}
		seen[input] = true
		if _, public := publicInputs[input]; !public {
			return fmt.Errorf("shape %s action %s declares unknown public input %s", shape, action, input)
		}
		if _, declared := program.Inputs[input]; !declared {
			return fmt.Errorf("shape %s action %s input %s is absent from program %s", shape, action, input, contract.Program)
		}
	}
	return nil
}

func parseValueType(value string) (valueType, bool) {
	switch value {
	case "number":
		return typeNumber, true
	case "string":
		return typeString, true
	case "boolean":
		return typeBoolean, true
	default:
		return "", false
	}
}

func sortedBindingKeys(bindings map[string]Binding) []string {
	keys := make([]string, 0, len(bindings))
	for key := range bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
