package eqcontrolgraph

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type CompileRequest struct {
	SectionKey string
	Shape      string
	Action     string
	Inputs     map[string]interface{}
}

type Target struct {
	BindingKey string
	Binding    Binding
	Role       string
	Phase      string
	Number     *float64
	EnumKey    string
	EnumValue  *EnumValue
}

type CompileResult struct {
	SectionKey    string
	Shape         string
	Action        string
	Targets       []Target
	AppliedInputs []string
}

func (document Document) Compile(request CompileRequest) (CompileResult, error) {
	if err := document.Validate(); err != nil {
		return CompileResult{}, err
	}
	section, ok := document.Section(request.SectionKey)
	if !ok {
		return CompileResult{}, fmt.Errorf("section %s is absent", request.SectionKey)
	}
	shape, ok := section.Shapes[request.Shape]
	if !ok {
		return CompileResult{}, fmt.Errorf("shape %s is unavailable in section %s", request.Shape, request.SectionKey)
	}
	contract, ok := shape.Actions[request.Action]
	if !ok {
		return CompileResult{}, fmt.Errorf("action %s is unavailable for %s in section %s", request.Action, request.Shape, request.SectionKey)
	}
	program := section.Programs[contract.Program]
	allowed := map[string]bool{}
	for _, input := range contract.RequiredInputs {
		allowed[input] = true
	}
	for _, input := range contract.OptionalInputs {
		allowed[input] = true
	}
	for input, value := range request.Inputs {
		if value == nil {
			continue
		}
		if !allowed[input] {
			return CompileResult{}, fmt.Errorf("input %s is not accepted by %s %s", input, request.Shape, request.Action)
		}
	}
	for _, input := range contract.RequiredInputs {
		if value, present := request.Inputs[input]; !present || value == nil {
			return CompileResult{}, fmt.Errorf("required input %s is absent", input)
		}
	}
	evaluationInputs := map[string]evalInput{
		"shape":  {present: true, value: graphValue{kind: typeString, text: request.Shape}},
		"action": {present: true, value: graphValue{kind: typeString, text: request.Action}},
	}
	applied := []string{}
	for name, spec := range program.Inputs {
		raw, present := request.Inputs[name]
		if !present || raw == nil {
			evaluationInputs[name] = evalInput{}
			continue
		}
		value, err := graphValueFromAny(raw)
		if err != nil {
			return CompileResult{}, fmt.Errorf("input %s: %w", name, err)
		}
		expected, _ := parseValueType(spec.Type)
		if value.kind != expected {
			return CompileResult{}, fmt.Errorf("input %s is %s, want %s", name, value.kind, expected)
		}
		evaluationInputs[name] = evalInput{present: true, value: value}
		applied = append(applied, name)
	}
	targets := make([]Target, 0, len(program.Writes))
	parameterTargets := map[string]Target{}
	for index, write := range program.Writes {
		if write.When != nil {
			nodes := 0
			condition, err := evalExpression(*write.When, evaluationInputs, 0, &nodes)
			if err != nil {
				return CompileResult{}, fmt.Errorf("write %d condition: %w", index, err)
			}
			if condition.kind != typeBoolean {
				return CompileResult{}, fmt.Errorf("write %d condition is not boolean", index)
			}
			if !condition.boolean {
				continue
			}
		}
		nodes := 0
		value, err := evalExpression(write.Value, evaluationInputs, 0, &nodes)
		if err != nil {
			return CompileResult{}, fmt.Errorf("write %d value: %w", index, err)
		}
		bindingKey := section.Bindings[write.Binding]
		binding := document.Bindings[bindingKey]
		target := Target{BindingKey: bindingKey, Binding: binding, Role: write.Role, Phase: write.Phase}
		if binding.Kind == "number" {
			if value.kind != typeNumber {
				return CompileResult{}, fmt.Errorf("write %d produced %s for numeric binding", index, value.kind)
			}
			if !physicalInRange(value.number, binding.Domain.Minimum, binding.Domain.Maximum) {
				return CompileResult{}, fmt.Errorf("write %d target %g is outside binding %s domain [%g,%g]", index, value.number, bindingKey, binding.Domain.Minimum, binding.Domain.Maximum)
			}
			number := value.number
			target.Number = &number
		} else {
			if value.kind != typeString {
				return CompileResult{}, fmt.Errorf("write %d produced %s for enum binding", index, value.kind)
			}
			enumValue, found := binding.Values[value.text]
			if !found {
				return CompileResult{}, fmt.Errorf("write %d enum key %q is absent from binding %s", index, value.text, bindingKey)
			}
			target.EnumKey = value.text
			target.EnumValue = &enumValue
		}
		if previous, exists := parameterTargets[binding.ParameterID]; exists {
			if !sameTarget(previous, target) {
				return CompileResult{}, fmt.Errorf("program produces conflicting targets for parameter %s", binding.ParameterID)
			}
			continue
		}
		parameterTargets[binding.ParameterID] = target
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return CompileResult{}, fmt.Errorf("program produced no parameter targets")
	}
	sort.SliceStable(targets, func(i, j int) bool {
		return targets[i].Phase != "activation" && targets[j].Phase == "activation"
	})
	sort.Strings(applied)
	return CompileResult{SectionKey: section.SectionKey, Shape: request.Shape, Action: request.Action,
		Targets: targets, AppliedInputs: applied}, nil
}

func (document Document) Section(key string) (Section, bool) {
	for _, section := range document.Sections {
		if section.SectionKey == key {
			return section, true
		}
	}
	return Section{}, false
}

func physicalInRange(value, minimum, maximum float64) bool {
	margin := 1e-9 * math.Max(1, math.Max(math.Abs(minimum), math.Abs(maximum)))
	return value >= minimum-margin && value <= maximum+margin
}

func sameTarget(left, right Target) bool {
	if left.Binding.ParameterID != right.Binding.ParameterID || left.Role != right.Role || left.Phase != right.Phase {
		return false
	}
	if left.Number != nil && right.Number != nil {
		return math.Abs(*left.Number-*right.Number) <= 1e-12
	}
	if left.EnumValue != nil && right.EnumValue != nil {
		return left.EnumKey == right.EnumKey
	}
	return false
}

func PublicInputNames() []string {
	keys := make([]string, 0, len(publicInputs))
	for key := range publicInputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func IsPublicShape(shape string) bool { return publicShapes[strings.TrimSpace(shape)] }
