package plugingrabber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/eqcontrolgraph"
)

func TestControlGraphDisplayNumberIsUnitAware(t *testing.T) {
	if value, ok := controlGraphDisplayNumber("3.82k", "Hz"); !ok || value != 3820 {
		t.Fatalf("frequency=%g ok=%v", value, ok)
	}
	if value, ok := controlGraphDisplayNumber("3.82k", "dB"); !ok || value != 3.82 {
		t.Fatalf("non-frequency=%g ok=%v", value, ok)
	}
	if value, ok := controlGraphDisplayNumber("-3,5 dB", "dB"); !ok || value != -3.5 {
		t.Fatalf("localized gain=%g ok=%v", value, ok)
	}
}

func TestConfiguredControlGraphLoadsAndUnloadsWithoutChangingRecognizer(t *testing.T) {
	document, digest := controlGraphAdapterFixture()
	directory := t.TempDir()
	documentPath := filepath.Join(directory, "fixture.vps.json")
	if err := eqcontrolgraph.SaveDocument(documentPath, document); err != nil {
		t.Fatal(err)
	}
	hash, _ := eqcontrolgraph.HashDocument(document)
	checks, err := eqcontrolgraph.ValidateLive(document, eqControlGraphLiveSurface(digest))
	if err != nil {
		t.Fatal(err)
	}
	if err := eqcontrolgraph.SaveAttestation(eqcontrolgraph.AttestationPath(documentPath), eqcontrolgraph.Attestation{
		SchemaVersion: eqcontrolgraph.AttestationVersion, VPSHash: hash, SurfaceSignature: document.SurfaceSignature,
		Status: "candidate", StaticChecks: checks,
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(eqcontrolgraph.DirectoryOverrideEnv, directory)
	t.Setenv(eqcontrolgraph.AllowCandidateEnv, "1")

	summary, err := BuildEQBandSummaryWithConfiguredControlGraph(digest)
	if err != nil {
		t.Fatal(err)
	}
	if firstNonEmptySummaryText(summary, "mapping_source") != "vps_control_graph" {
		t.Fatalf("summary=%#v", summary)
	}
	sections, _ := summary["sections"].([]map[string]any)
	if len(sections) != 1 {
		t.Fatalf("sections=%#v", summary["sections"])
	}
	runtime, ok := sections[0]["control_graph_runtime"].(*EQControlGraphRuntime)
	if !ok || runtime == nil {
		t.Fatalf("runtime=%#v", sections[0]["control_graph_runtime"])
	}
	result, err := runtime.Compile("bell", "upsert", map[string]interface{}{"frequency_hz": 1000.0, "gain_db": -3.0})
	if err != nil || len(result.Targets) != 5 {
		t.Fatalf("result=%#v err=%v", result, err)
	}

	if err = os.Remove(documentPath); err != nil {
		t.Fatal(err)
	}
	summary, err = BuildEQBandSummaryWithConfiguredControlGraph(digest)
	if err != nil {
		t.Fatal(err)
	}
	if summary != nil {
		t.Fatalf("unload should restore fixture's nil generic baseline: %#v", summary)
	}
}

func TestMatchingInvalidControlGraphFailsClosed(t *testing.T) {
	_, digest := controlGraphAdapterFixture()
	directory := t.TempDir()
	t.Setenv(eqcontrolgraph.DirectoryOverrideEnv, directory)
	t.Setenv(eqcontrolgraph.AllowCandidateEnv, "1")
	raw := `{
  "schema_version":"vit.eq_vps.control_graph.v1",
  "plugin":{"name":"Fixture EQ","format":"VST3","version":"1.0","manufacturer":"Fixture Maker"},
  "surface_signature":"p_fixture",
  "unknown_executable_field":"run"
}`
	if err := os.WriteFile(filepath.Join(directory, "invalid.vps.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildEQBandSummaryWithConfiguredControlGraph(digest); err == nil || !strings.Contains(err.Error(), "matching VPS") {
		t.Fatalf("error=%v", err)
	}
	if _, err := BuildContextPackWithConfiguredControlGraph(digest); err == nil || !strings.Contains(err.Error(), "matching VPS") {
		t.Fatalf("context pack error=%v", err)
	}
}

func TestControlGraphGenerationRejectsMalformedHash(t *testing.T) {
	document, digest := controlGraphAdapterFixture()
	resolved := &eqcontrolgraph.ResolvedDocument{Document: document,
		Attestation: eqcontrolgraph.Attestation{VPSHash: "short", Status: "candidate"}}
	if _, err := BuildEQBandSummaryWithControlGraph(digest, resolved); err == nil || !strings.Contains(err.Error(), "invalid vps_hash") {
		t.Fatalf("error=%v", err)
	}
}

func controlGraphAdapterFixture() (eqcontrolgraph.Document, ParameterDigest) {
	present := func(name string) *eqcontrolgraph.Expression {
		expression := eqcontrolgraph.Expression{Op: "present", Input: name}
		return &expression
	}
	input := func(name string) eqcontrolgraph.Expression {
		return eqcontrolgraph.Expression{Op: "input", Input: name}
	}
	constant := func(value interface{}) eqcontrolgraph.Expression {
		return eqcontrolgraph.Expression{Op: "constant", Value: value}
	}
	less := eqcontrolgraph.Expression{Op: "less_than", Args: []eqcontrolgraph.Expression{input("gain_db"), constant(0.0)}}
	polarity := eqcontrolgraph.Expression{Op: "select", Condition: &less,
		Then: graphExpressionPointer(constant("cut")), Else: graphExpressionPointer(constant("boost"))}
	document := eqcontrolgraph.Document{
		SchemaVersion:    eqcontrolgraph.SchemaVersion,
		Plugin:           eqcontrolgraph.PluginIdentity{Name: "Fixture EQ", Manufacturer: "Fixture Maker", Format: "VST3", Version: "1.0"},
		SurfaceSignature: "p_fixture", PublicClassification: "fixed_slot_adjustable", ChannelContract: "shared",
		Bindings: map[string]eqcontrolgraph.Binding{
			"frequency": controlGraphNumericBinding("freq", "Frequency", "Hz", 100, 10000, [][2]float64{{0, 100}, {.5, 1000}, {1, 10000}}, "logarithmic"),
			"gain":      controlGraphNumericBinding("gain", "Gain", "dB", 0, 12, [][2]float64{{0, 0}, {.5, 6}, {1, 12}}, "linear"),
			"mode":      controlGraphEnumBinding("mode", "Mode", map[string]eqcontrolgraph.EnumValue{"boost": {Normalized: 0, Label: "Boost"}, "cut": {Normalized: 1, Label: "Cut"}}),
			"shape":     controlGraphEnumBinding("shape", "Shape", map[string]eqcontrolgraph.EnumValue{"bell": {Normalized: 0, Label: "Bell"}}),
			"active":    controlGraphEnumBinding("active", "Active", map[string]eqcontrolgraph.EnumValue{"out": {Normalized: 0, Label: "Out"}, "in": {Normalized: 1, Label: "In"}}),
		},
	}
	document.Sections = []eqcontrolgraph.Section{{
		SectionKey: "top", Addressing: "resident", Activation: eqcontrolgraph.Activation{State: "explicit"},
		Selection: eqcontrolgraph.Selection{FrequencyBinding: "frequency"},
		Bindings:  map[string]string{"frequency": "frequency", "gain": "gain", "mode": "mode", "shape": "shape", "active": "active"},
		Shapes: map[string]eqcontrolgraph.Shape{"bell": {Actions: map[string]eqcontrolgraph.ActionContract{
			"upsert":  {Program: "set", RequiredInputs: []string{"frequency_hz", "gain_db"}},
			"modify":  {Program: "set", OptionalInputs: []string{"frequency_hz", "gain_db"}},
			"disable": {Program: "disable"},
		}}},
		Programs: map[string]eqcontrolgraph.Program{
			"set": {Inputs: map[string]eqcontrolgraph.InputSpec{"frequency_hz": {Type: "number"}, "gain_db": {Type: "number"}}, Writes: []eqcontrolgraph.ProgramWrite{
				{Binding: "shape", Role: "shape", Value: input("shape")},
				{Binding: "frequency", Role: "frequency", When: present("frequency_hz"), Value: input("frequency_hz")},
				{Binding: "gain", Role: "gain", When: present("gain_db"), Value: eqcontrolgraph.Expression{Op: "abs", Args: []eqcontrolgraph.Expression{input("gain_db")}}},
				{Binding: "mode", Role: "gain_polarity", When: present("gain_db"), Value: polarity},
				{Binding: "active", Role: "activation", Phase: "activation", Value: constant("in")},
			}},
			"disable": {Writes: []eqcontrolgraph.ProgramWrite{{Binding: "active", Role: "activation", Phase: "activation", Value: constant("out")}}},
		},
	}}
	digest := ParameterDigest{PluginName: document.Plugin.Name, PluginIdentity: map[string]any{
		"manufacturer": document.Plugin.Manufacturer, "plugin_format": document.Plugin.Format, "version": document.Plugin.Version,
	}, CurrentParamSignatureHash: document.SurfaceSignature}
	for _, key := range []string{"frequency", "gain", "mode", "shape", "active"} {
		binding := document.Bindings[key]
		parameter := ParameterInfo{ID: binding.ParameterID, Name: binding.ParameterName, NormalizedValue: 0.0,
			DisplayProbe: &ParameterDisplayProbe{}}
		if binding.Kind == "number" {
			parameter.DisplayDomainCandidate = &PluginDisplayDomain{Unit: binding.Domain.Unit}
			for _, point := range binding.Domain.Curve {
				parameter.DisplayProbe.Samples = append(parameter.DisplayProbe.Samples, ParameterDisplayProbeSample{
					NormalizedValue: point[0], Text: controlGraphFixtureText(point[1], binding.Domain.Unit),
				})
			}
		} else {
			for _, value := range binding.Values {
				parameter.DisplayProbe.DiscreteLabels = append(parameter.DisplayProbe.DiscreteLabels, ParameterDisplayProbeLabel{
					Value: value.Normalized, Label: value.Label,
				})
			}
		}
		digest.Parameters = append(digest.Parameters, parameter)
	}
	return document, digest
}

func controlGraphNumericBinding(id, name, unit string, minimum, maximum float64, curve [][2]float64, law string) eqcontrolgraph.Binding {
	return eqcontrolgraph.Binding{Kind: "number", ParameterID: id, ParameterName: name, Channel: "shared",
		Domain: &eqcontrolgraph.NumericDomain{Unit: unit, Minimum: minimum, Maximum: maximum, Curve: curve, ValueLaw: law}}
}

func controlGraphEnumBinding(id, name string, values map[string]eqcontrolgraph.EnumValue) eqcontrolgraph.Binding {
	return eqcontrolgraph.Binding{Kind: "enum", ParameterID: id, ParameterName: name, Channel: "shared", Values: values}
}

func controlGraphFixtureText(value float64, unit string) string {
	if unit == "Hz" {
		return fmt.Sprintf("%g Hz", value)
	}
	return fmt.Sprintf("%g", value)
}

func graphExpressionPointer(expression eqcontrolgraph.Expression) *eqcontrolgraph.Expression {
	return &expression
}

func firstNonEmptySummaryText(row map[string]any, key string) string {
	value, _ := row[key].(string)
	return value
}
