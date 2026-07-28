package eqcontrolgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeclarativeCompoundGainCompilesWithoutNamedStructure(t *testing.T) {
	document := compoundGainFixture()
	result, err := document.Compile(CompileRequest{SectionKey: "top", Shape: "bell", Action: "upsert",
		Inputs: map[string]interface{}{"frequency_hz": 3400.0, "gain_db": -3.0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 5 {
		t.Fatalf("targets=%#v", result.Targets)
	}
	byRole := map[string]Target{}
	for _, target := range result.Targets {
		byRole[target.Role] = target
	}
	if byRole["gain"].Number == nil || *byRole["gain"].Number != 3 {
		t.Fatalf("gain target=%#v", byRole["gain"])
	}
	if byRole["gain_polarity"].EnumKey != "cut" || byRole["gain_polarity"].EnumValue.Label != "Cut" {
		t.Fatalf("polarity target=%#v", byRole["gain_polarity"])
	}
	if result.Targets[len(result.Targets)-1].Phase != "activation" {
		t.Fatalf("activation was not ordered last: %#v", result.Targets)
	}
	if strings.Contains(strings.ToLower(documentText(t, document)), "signed_magnitude") {
		t.Fatal("document contains a named compound structure")
	}
}

func TestDirectBipolarGainUsesSameCompiler(t *testing.T) {
	document := directGainFixture()
	result, err := document.Compile(CompileRequest{SectionKey: "band1", Shape: "bell", Action: "upsert",
		Inputs: map[string]interface{}{"frequency_hz": 1000.0, "gain_db": -3.0, "q": 0.5}})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range result.Targets {
		if target.Role == "gain" && (target.Number == nil || *target.Number != -3) {
			t.Fatalf("direct gain target=%#v", target)
		}
	}
}

func TestActionContractRejectsUndeclaredExplicitInput(t *testing.T) {
	document := compoundGainFixture()
	_, err := document.Compile(CompileRequest{SectionKey: "top", Shape: "bell", Action: "upsert",
		Inputs: map[string]interface{}{"frequency_hz": 3400.0, "gain_db": -3.0, "q": 0.5}})
	if err == nil || !strings.Contains(err.Error(), "input q is not accepted") {
		t.Fatalf("error=%v", err)
	}
}

func TestExpressionLanguageFailsClosed(t *testing.T) {
	t.Run("unknown operator", func(t *testing.T) {
		document := compoundGainFixture()
		document.Sections[0].Programs["set"] = Program{Inputs: map[string]InputSpec{"frequency_hz": {Type: "number"}, "gain_db": {Type: "number"}},
			Writes: []ProgramWrite{{Binding: "gain", Role: "gain", Value: Expression{Op: "run_script", Value: "calc.exe"}}}}
		if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "unknown expression op") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("depth", func(t *testing.T) {
		document := compoundGainFixture()
		expression := Expression{Op: "input", Input: "gain_db"}
		for index := 0; index < 40; index++ {
			expression = Expression{Op: "abs", Args: []Expression{expression}}
		}
		program := document.Sections[0].Programs["set"]
		program.Writes[2].Value = expression
		document.Sections[0].Programs["set"] = program
		if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "maximum depth") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("unknown JSON field", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.vps.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":"vit.eq_vps.control_graph.v1","arbitrary_script":"x"}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDocument(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestFreshSurfaceAndAttestationAreRequired(t *testing.T) {
	document := compoundGainFixture()
	surface := liveSurfaceFor(document)
	checks, err := ValidateLive(document, surface)
	if err != nil || checks <= 0 {
		t.Fatalf("checks=%d err=%v", checks, err)
	}

	stale := surface
	stale.Signature = "stale"
	if _, err = ValidateLive(document, stale); err == nil || !strings.Contains(err.Error(), "surface_signature") {
		t.Fatalf("stale signature error=%v", err)
	}
	wrongVersion := liveSurfaceFor(document)
	wrongVersion.Plugin.Version = "2.0"
	if _, err = ValidateLive(document, wrongVersion); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("version mismatch error=%v", err)
	}
	missingManufacturer := liveSurfaceFor(document)
	document.Plugin.Manufacturer = "Fixture Maker"
	missingManufacturer.Plugin.Manufacturer = ""
	if _, err = ValidateLive(document, missingManufacturer); err == nil || !strings.Contains(err.Error(), "manufacturer") {
		t.Fatalf("manufacturer mismatch error=%v", err)
	}
	document.Plugin.Manufacturer = ""
	contradictory := liveSurfaceFor(document)
	for index := range contradictory.Parameters {
		if len(contradictory.Parameters[index].Samples) > 0 {
			contradictory.Parameters[index].Samples[0].Physical++
			break
		}
	}
	if _, err = ValidateLive(document, contradictory); err == nil || !strings.Contains(err.Error(), "curve point") {
		t.Fatalf("curve mismatch error=%v", err)
	}

	directory := t.TempDir()
	documentPath := filepath.Join(directory, "fixture.vps.json")
	if err = SaveDocument(documentPath, document); err != nil {
		t.Fatal(err)
	}
	hash, _ := HashDocument(document)
	candidate := Attestation{SchemaVersion: AttestationVersion, VPSHash: hash,
		SurfaceSignature: document.SurfaceSignature, Status: "candidate", StaticChecks: checks}
	if err = SaveAttestation(AttestationPath(documentPath), candidate); err != nil {
		t.Fatal(err)
	}
	if _, _, err = ResolveDirectory(directory, surface, false); err == nil || !strings.Contains(err.Error(), "not installable") {
		t.Fatalf("candidate resolve error=%v", err)
	}
	resolved, warnings, err := ResolveDirectory(directory, surface, true)
	if err != nil || len(warnings) != 0 || resolved == nil {
		t.Fatalf("candidate resolved=%#v warnings=%v err=%v", resolved, warnings, err)
	}

	verified := candidate
	verified.Status = "verified"
	verified.VerifiedAt = time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC)
	verified.StaticChecks = checks
	verified.LiveChecks = LiveCheckEvidence{ApplyReadback: true, FormalUndoZeroDrift: true, UnloadRestoresBaseline: true}
	if err = SaveAttestation(AttestationPath(documentPath), verified); err != nil {
		t.Fatal(err)
	}
	resolved, warnings, err = ResolveDirectory(directory, surface, false)
	if err != nil || len(warnings) != 0 || resolved == nil {
		t.Fatalf("verified resolved=%#v warnings=%v err=%v", resolved, warnings, err)
	}
	if err = os.Remove(documentPath); err != nil {
		t.Fatal(err)
	}
	resolved, _, err = ResolveDirectory(directory, surface, false)
	if err != nil || resolved != nil {
		t.Fatalf("unloaded resolved=%#v err=%v", resolved, err)
	}
}

func compoundGainFixture() Document {
	present := func(input string) *Expression { value := Expression{Op: "present", Input: input}; return &value }
	constant := func(value interface{}) Expression { return Expression{Op: "constant", Value: value} }
	input := func(name string) Expression { return Expression{Op: "input", Input: name} }
	lessThan := Expression{Op: "less_than", Args: []Expression{input("gain_db"), constant(0.0)}}
	selectPolarity := Expression{Op: "select", Condition: &lessThan,
		Then: expressionPointer(constant("cut")), Else: expressionPointer(constant("boost"))}
	return Document{SchemaVersion: SchemaVersion,
		Plugin:           PluginIdentity{Name: "Fixture EQ", Format: "VST3", Version: "1.0"},
		SurfaceSignature: "p_fixture", PublicClassification: "fixed_slot_adjustable", ChannelContract: "shared",
		Bindings: map[string]Binding{
			"frequency": numericBinding("freq", "Frequency", "Hz", 100, 10000, [][2]float64{{0, 100}, {.5, 1000}, {1, 10000}}),
			"gain":      numericBinding("gain", "Gain", "dB", 0, 15, [][2]float64{{0, 0}, {.5, 6}, {1, 15}}),
			"mode":      enumBinding("mode", "Mode", map[string]EnumValue{"boost": {Normalized: 0, Label: "Boost"}, "cut": {Normalized: 1, Label: "Cut"}}),
			"shape":     enumBinding("shape", "Shape", map[string]EnumValue{"bell": {Normalized: 0, Label: "Bell"}}),
			"active":    enumBinding("active", "Active", map[string]EnumValue{"out": {Normalized: 0, Label: "Out"}, "in": {Normalized: 1, Label: "In"}}),
		},
		Sections: []Section{{SectionKey: "top", Addressing: "resident", Activation: Activation{State: "explicit"},
			Selection: Selection{FrequencyBinding: "frequency"},
			Bindings:  map[string]string{"frequency": "frequency", "gain": "gain", "mode": "mode", "shape": "shape", "active": "active"},
			Shapes: map[string]Shape{"bell": {Actions: map[string]ActionContract{
				"upsert":  {Program: "set", RequiredInputs: []string{"frequency_hz", "gain_db"}},
				"modify":  {Program: "set", OptionalInputs: []string{"frequency_hz", "gain_db"}},
				"disable": {Program: "disable"},
			}}},
			Programs: map[string]Program{
				"set": {Inputs: map[string]InputSpec{"frequency_hz": {Type: "number"}, "gain_db": {Type: "number"}}, Writes: []ProgramWrite{
					{Binding: "shape", Role: "shape", Value: input("shape")},
					{Binding: "frequency", Role: "freq", When: present("frequency_hz"), Value: input("frequency_hz")},
					{Binding: "gain", Role: "gain", When: present("gain_db"), Value: Expression{Op: "abs", Args: []Expression{input("gain_db")}}},
					{Binding: "mode", Role: "gain_polarity", When: present("gain_db"), Value: selectPolarity},
					{Binding: "active", Role: "used", Phase: "activation", Value: constant("in")},
				}},
				"disable": {Writes: []ProgramWrite{{Binding: "active", Role: "disabled", Phase: "activation", Value: constant("out")}}},
			},
		}},
	}
}

func directGainFixture() Document {
	document := compoundGainFixture()
	document.Sections[0].SectionKey = "band1"
	document.Bindings["gain"] = numericBinding("gain", "Gain", "dB", -18, 18, [][2]float64{{0, -18}, {.5, 0}, {1, 18}})
	delete(document.Bindings, "mode")
	delete(document.Sections[0].Bindings, "mode")
	program := document.Sections[0].Programs["set"]
	program.Inputs["q"] = InputSpec{Type: "number"}
	program.Writes = []ProgramWrite{
		program.Writes[0], program.Writes[1],
		{Binding: "gain", Role: "gain", When: expressionPointer(Expression{Op: "present", Input: "gain_db"}), Value: Expression{Op: "input", Input: "gain_db"}},
		program.Writes[4],
	}
	document.Bindings["q"] = numericBinding("q", "Q", "", .5, 100, [][2]float64{{0, .5}, {.5, 7.1}, {1, 100}})
	document.Sections[0].Bindings["q"] = "q"
	program.Writes = append(program.Writes[:3], ProgramWrite{Binding: "q", Role: "q", When: expressionPointer(Expression{Op: "present", Input: "q"}), Value: Expression{Op: "input", Input: "q"}}, program.Writes[3])
	document.Sections[0].Programs["set"] = program
	shape := document.Sections[0].Shapes["bell"]
	upsert := shape.Actions["upsert"]
	upsert.RequiredInputs = append(upsert.RequiredInputs, "q")
	shape.Actions["upsert"] = upsert
	modify := shape.Actions["modify"]
	modify.OptionalInputs = append(modify.OptionalInputs, "q")
	shape.Actions["modify"] = modify
	document.Sections[0].Shapes["bell"] = shape
	return document
}

func numericBinding(id, name, unit string, minimum, maximum float64, curve [][2]float64) Binding {
	return Binding{Kind: "number", ParameterID: id, ParameterName: name, Channel: "shared",
		Domain: &NumericDomain{Unit: unit, Minimum: minimum, Maximum: maximum, Curve: curve}}
}

func enumBinding(id, name string, values map[string]EnumValue) Binding {
	return Binding{Kind: "enum", ParameterID: id, ParameterName: name, Channel: "shared", Values: values}
}

func expressionPointer(expression Expression) *Expression { return &expression }

func liveSurfaceFor(document Document) LiveSurface {
	surface := LiveSurface{Plugin: document.Plugin, Signature: document.SurfaceSignature}
	for _, key := range sortedBindingKeys(document.Bindings) {
		binding := document.Bindings[key]
		parameter := LiveParameter{ID: binding.ParameterID, Name: binding.ParameterName, HostControllable: true}
		if binding.Kind == "number" {
			for _, point := range binding.Domain.Curve {
				parameter.Samples = append(parameter.Samples, LiveNumericSample{Normalized: point[0], Physical: point[1]})
			}
		} else {
			for _, value := range binding.Values {
				parameter.EnumValues = append(parameter.EnumValues, LiveEnumValue{Normalized: value.Normalized, Label: value.Label})
			}
		}
		surface.Parameters = append(surface.Parameters, parameter)
	}
	return surface
}

func documentText(t *testing.T, document Document) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.vps.json")
	if err := SaveDocument(path, document); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
