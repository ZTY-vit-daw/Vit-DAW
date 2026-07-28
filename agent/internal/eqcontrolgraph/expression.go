package eqcontrolgraph

import (
	"fmt"
	"math"
	"strings"
)

type valueType string

const (
	typeNumber  valueType = "number"
	typeString  valueType = "string"
	typeBoolean valueType = "boolean"
)

type graphValue struct {
	kind    valueType
	number  float64
	text    string
	boolean bool
}

type evalInput struct {
	present bool
	value   graphValue
}

func inferExpressionType(expression Expression, inputs map[string]valueType, depth int, nodes *int) (valueType, error) {
	if depth > 32 {
		return "", fmt.Errorf("expression exceeds maximum depth 32")
	}
	*nodes++
	if *nodes > 256 {
		return "", fmt.Errorf("expression exceeds maximum node count 256")
	}
	op := strings.TrimSpace(expression.Op)
	switch op {
	case "input":
		kind, ok := inputs[expression.Input]
		if !ok {
			return "", fmt.Errorf("input %q is not declared", expression.Input)
		}
		return kind, nil
	case "constant":
		value, err := graphValueFromAny(expression.Value)
		return value.kind, err
	case "present":
		if _, ok := inputs[expression.Input]; !ok {
			return "", fmt.Errorf("input %q is not declared", expression.Input)
		}
		return typeBoolean, nil
	case "abs", "negate":
		if err := expectArgs(expression, 1); err != nil {
			return "", err
		}
		return inferNumericArgs(expression.Args, inputs, depth, nodes)
	case "add", "subtract", "multiply", "divide", "minimum", "maximum":
		if err := expectArgs(expression, 2); err != nil {
			return "", err
		}
		return inferNumericArgs(expression.Args, inputs, depth, nodes)
	case "clamp":
		if err := expectArgs(expression, 3); err != nil {
			return "", err
		}
		return inferNumericArgs(expression.Args, inputs, depth, nodes)
	case "less_than", "less_or_equal", "greater_than", "greater_or_equal":
		if err := expectArgs(expression, 2); err != nil {
			return "", err
		}
		if _, err := inferNumericArgs(expression.Args, inputs, depth, nodes); err != nil {
			return "", err
		}
		return typeBoolean, nil
	case "equal":
		if err := expectArgs(expression, 2); err != nil {
			return "", err
		}
		left, err := inferExpressionType(expression.Args[0], inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		right, err := inferExpressionType(expression.Args[1], inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if left != right {
			return "", fmt.Errorf("equal operands have different types %s and %s", left, right)
		}
		return typeBoolean, nil
	case "and", "or":
		if len(expression.Args) < 2 {
			return "", fmt.Errorf("%s requires at least two args", op)
		}
		for _, argument := range expression.Args {
			kind, err := inferExpressionType(argument, inputs, depth+1, nodes)
			if err != nil {
				return "", err
			}
			if kind != typeBoolean {
				return "", fmt.Errorf("%s requires boolean args, got %s", op, kind)
			}
		}
		return typeBoolean, nil
	case "not":
		if err := expectArgs(expression, 1); err != nil {
			return "", err
		}
		kind, err := inferExpressionType(expression.Args[0], inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if kind != typeBoolean {
			return "", fmt.Errorf("not requires a boolean arg, got %s", kind)
		}
		return typeBoolean, nil
	case "select":
		if expression.Condition == nil || expression.Then == nil || expression.Else == nil {
			return "", fmt.Errorf("select requires condition, then, and else")
		}
		condition, err := inferExpressionType(*expression.Condition, inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if condition != typeBoolean {
			return "", fmt.Errorf("select condition must be boolean")
		}
		thenType, err := inferExpressionType(*expression.Then, inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		elseType, err := inferExpressionType(*expression.Else, inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if thenType != elseType {
			return "", fmt.Errorf("select branches have different types %s and %s", thenType, elseType)
		}
		return thenType, nil
	case "lookup":
		if expression.Key == nil || len(expression.Cases) == 0 {
			return "", fmt.Errorf("lookup requires key and cases")
		}
		keyType, err := inferExpressionType(*expression.Key, inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if keyType != typeString {
			return "", fmt.Errorf("lookup key must be string")
		}
		var resultType valueType
		for key, candidate := range expression.Cases {
			if strings.TrimSpace(key) == "" {
				return "", fmt.Errorf("lookup has an empty case key")
			}
			candidateType, candidateErr := inferExpressionType(candidate, inputs, depth+1, nodes)
			if candidateErr != nil {
				return "", candidateErr
			}
			if resultType == "" {
				resultType = candidateType
			} else if resultType != candidateType {
				return "", fmt.Errorf("lookup cases have different types %s and %s", resultType, candidateType)
			}
		}
		if expression.Default != nil {
			defaultType, defaultErr := inferExpressionType(*expression.Default, inputs, depth+1, nodes)
			if defaultErr != nil {
				return "", defaultErr
			}
			if resultType != defaultType {
				return "", fmt.Errorf("lookup default type %s differs from case type %s", defaultType, resultType)
			}
		}
		return resultType, nil
	default:
		return "", fmt.Errorf("unknown expression op %q", op)
	}
}

func inferNumericArgs(arguments []Expression, inputs map[string]valueType, depth int, nodes *int) (valueType, error) {
	for _, argument := range arguments {
		kind, err := inferExpressionType(argument, inputs, depth+1, nodes)
		if err != nil {
			return "", err
		}
		if kind != typeNumber {
			return "", fmt.Errorf("numeric operation received %s", kind)
		}
	}
	return typeNumber, nil
}

func expectArgs(expression Expression, count int) error {
	if len(expression.Args) != count {
		return fmt.Errorf("%s requires %d args", expression.Op, count)
	}
	return nil
}

func evalExpression(expression Expression, inputs map[string]evalInput, depth int, nodes *int) (graphValue, error) {
	if depth > 32 {
		return graphValue{}, fmt.Errorf("expression exceeds maximum depth 32")
	}
	*nodes++
	if *nodes > 256 {
		return graphValue{}, fmt.Errorf("expression exceeds maximum node count 256")
	}
	op := strings.TrimSpace(expression.Op)
	switch op {
	case "input":
		input, ok := inputs[expression.Input]
		if !ok || !input.present {
			return graphValue{}, fmt.Errorf("input %q is absent", expression.Input)
		}
		return input.value, nil
	case "constant":
		return graphValueFromAny(expression.Value)
	case "present":
		input, ok := inputs[expression.Input]
		return graphValue{kind: typeBoolean, boolean: ok && input.present}, nil
	case "abs", "negate":
		values, err := evalNumericArgs(expression.Args, inputs, depth, nodes)
		if err != nil {
			return graphValue{}, err
		}
		result := values[0]
		if op == "abs" {
			result = math.Abs(result)
		} else {
			result = -result
		}
		return graphValue{kind: typeNumber, number: result}, nil
	case "add", "subtract", "multiply", "divide", "minimum", "maximum", "clamp":
		values, err := evalNumericArgs(expression.Args, inputs, depth, nodes)
		if err != nil {
			return graphValue{}, err
		}
		var result float64
		switch op {
		case "add":
			result = values[0] + values[1]
		case "subtract":
			result = values[0] - values[1]
		case "multiply":
			result = values[0] * values[1]
		case "divide":
			if values[1] == 0 {
				return graphValue{}, fmt.Errorf("division by zero")
			}
			result = values[0] / values[1]
		case "minimum":
			result = math.Min(values[0], values[1])
		case "maximum":
			result = math.Max(values[0], values[1])
		case "clamp":
			result = math.Max(values[1], math.Min(values[0], values[2]))
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return graphValue{}, fmt.Errorf("%s produced a non-finite value", op)
		}
		return graphValue{kind: typeNumber, number: result}, nil
	case "less_than", "less_or_equal", "greater_than", "greater_or_equal":
		values, err := evalNumericArgs(expression.Args, inputs, depth, nodes)
		if err != nil {
			return graphValue{}, err
		}
		result := false
		switch op {
		case "less_than":
			result = values[0] < values[1]
		case "less_or_equal":
			result = values[0] <= values[1]
		case "greater_than":
			result = values[0] > values[1]
		case "greater_or_equal":
			result = values[0] >= values[1]
		}
		return graphValue{kind: typeBoolean, boolean: result}, nil
	case "equal":
		left, err := evalExpression(expression.Args[0], inputs, depth+1, nodes)
		if err != nil {
			return graphValue{}, err
		}
		right, err := evalExpression(expression.Args[1], inputs, depth+1, nodes)
		if err != nil {
			return graphValue{}, err
		}
		if left.kind != right.kind {
			return graphValue{}, fmt.Errorf("equal operands have different types")
		}
		result := false
		switch left.kind {
		case typeNumber:
			result = left.number == right.number
		case typeString:
			result = left.text == right.text
		case typeBoolean:
			result = left.boolean == right.boolean
		}
		return graphValue{kind: typeBoolean, boolean: result}, nil
	case "and", "or":
		result := op == "and"
		for _, argument := range expression.Args {
			value, err := evalExpression(argument, inputs, depth+1, nodes)
			if err != nil {
				return graphValue{}, err
			}
			if value.kind != typeBoolean {
				return graphValue{}, fmt.Errorf("%s received non-boolean arg", op)
			}
			if op == "and" {
				result = result && value.boolean
			} else {
				result = result || value.boolean
			}
		}
		return graphValue{kind: typeBoolean, boolean: result}, nil
	case "not":
		value, err := evalExpression(expression.Args[0], inputs, depth+1, nodes)
		if err != nil {
			return graphValue{}, err
		}
		if value.kind != typeBoolean {
			return graphValue{}, fmt.Errorf("not received non-boolean arg")
		}
		return graphValue{kind: typeBoolean, boolean: !value.boolean}, nil
	case "select":
		condition, err := evalExpression(*expression.Condition, inputs, depth+1, nodes)
		if err != nil {
			return graphValue{}, err
		}
		if condition.kind != typeBoolean {
			return graphValue{}, fmt.Errorf("select condition is not boolean")
		}
		if condition.boolean {
			return evalExpression(*expression.Then, inputs, depth+1, nodes)
		}
		return evalExpression(*expression.Else, inputs, depth+1, nodes)
	case "lookup":
		key, err := evalExpression(*expression.Key, inputs, depth+1, nodes)
		if err != nil {
			return graphValue{}, err
		}
		if key.kind != typeString {
			return graphValue{}, fmt.Errorf("lookup key is not string")
		}
		candidate, ok := expression.Cases[key.text]
		if !ok {
			if expression.Default == nil {
				return graphValue{}, fmt.Errorf("lookup has no case for %q", key.text)
			}
			return evalExpression(*expression.Default, inputs, depth+1, nodes)
		}
		return evalExpression(candidate, inputs, depth+1, nodes)
	default:
		return graphValue{}, fmt.Errorf("unknown expression op %q", op)
	}
}

func evalNumericArgs(arguments []Expression, inputs map[string]evalInput, depth int, nodes *int) ([]float64, error) {
	values := make([]float64, 0, len(arguments))
	for _, argument := range arguments {
		value, err := evalExpression(argument, inputs, depth+1, nodes)
		if err != nil {
			return nil, err
		}
		if value.kind != typeNumber {
			return nil, fmt.Errorf("numeric operation received %s", value.kind)
		}
		values = append(values, value.number)
	}
	return values, nil
}

func graphValueFromAny(value interface{}) (graphValue, error) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return graphValue{}, fmt.Errorf("constant is non-finite")
		}
		return graphValue{kind: typeNumber, number: typed}, nil
	case float32:
		return graphValue{kind: typeNumber, number: float64(typed)}, nil
	case int:
		return graphValue{kind: typeNumber, number: float64(typed)}, nil
	case int64:
		return graphValue{kind: typeNumber, number: float64(typed)}, nil
	case string:
		return graphValue{kind: typeString, text: typed}, nil
	case bool:
		return graphValue{kind: typeBoolean, boolean: typed}, nil
	default:
		return graphValue{}, fmt.Errorf("unsupported value type %T", value)
	}
}
