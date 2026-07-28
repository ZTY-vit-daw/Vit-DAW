package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/eqcontrolgraph"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestControlGraphPlannerCompilesCompoundWritesIntoAtomicSteps(t *testing.T) {
	document := chatControlGraphFixture()
	runtime, err := plugingrabber.NewEQControlGraphRuntime(document, "top")
	if err != nil {
		t.Fatal(err)
	}
	summary := chatControlGraphSummary(runtime)
	gain := -3.0
	writes, selection, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 2000, GainDB: &gain, Shape: "bell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstNonEmptyText(selection, "mapping_source") != "vps_control_graph" || len(writes) != 5 {
		t.Fatalf("selection=%#v writes=%#v", selection, writes)
	}
	byRole := map[string]eqWriteStep{}
	for _, write := range writes {
		byRole[write.Role] = write
	}
	if write := byRole["gain"]; write.ParamID != "gain" || write.RequestedPhysical == nil ||
		*write.RequestedPhysical != 3 || write.NormalizedValue != .25 {
		t.Fatalf("gain=%#v", write)
	}
	if write := byRole["gain_polarity"]; write.ParamID != "mode" || write.ExpectedLabel != "Cut" || write.NormalizedValue != 1 {
		t.Fatalf("polarity=%#v", write)
	}
	if last := writes[len(writes)-1]; last.ParamID != "active" || last.Role != "used" || !last.ActivationLast {
		t.Fatalf("activation=%#v", last)
	}
}

func TestControlGraphPlannerRejectsUndeclaredExplicitQBeforeWrites(t *testing.T) {
	document := chatControlGraphFixture()
	runtime, err := plugingrabber.NewEQControlGraphRuntime(document, "top")
	if err != nil {
		t.Fatal(err)
	}
	gain, q := 3.0, .7
	_, _, err = eqTypedSectionWritePlan(chatControlGraphSummary(runtime), eqTypedPointRequest{
		FreqHz: 2000, GainDB: &gain, Q: &q, Shape: "bell",
	})
	if err == nil || !strings.Contains(err.Error(), "input q is not accepted") {
		t.Fatalf("error=%v", err)
	}
}

func TestControlGraphPlannerUsesProgramsForModifyAndDisable(t *testing.T) {
	document := chatControlGraphFixture()
	runtime, err := plugingrabber.NewEQControlGraphRuntime(document, "top")
	if err != nil {
		t.Fatal(err)
	}
	section := mapRowsValue(chatControlGraphSummary(runtime)["sections"])[0]
	gain := 6.0
	writes, _, err := planEQModifyWrites(section, eqEditRequest{Action: "modify", GainDB: &gain}, "bell")
	if err != nil {
		t.Fatal(err)
	}
	foundGain := false
	for _, write := range writes {
		foundGain = foundGain || write.Role == "gain" && write.RequestedPhysical != nil && *write.RequestedPhysical == 6
	}
	if !foundGain {
		t.Fatalf("modify writes=%#v", writes)
	}
	disable, err := eqControlGraphCompileWrites(section, "bell", "disable", nil)
	if err != nil || len(disable) != 1 || disable[0].Role != "disabled" || disable[0].ExpectedLabel != "Out" || !disable[0].ActivationLast {
		t.Fatalf("disable=%#v err=%v", disable, err)
	}
}

func chatControlGraphFixture() eqcontrolgraph.Document {
	pointer := func(expression eqcontrolgraph.Expression) *eqcontrolgraph.Expression { return &expression }
	input := func(name string) eqcontrolgraph.Expression {
		return eqcontrolgraph.Expression{Op: "input", Input: name}
	}
	constant := func(value interface{}) eqcontrolgraph.Expression {
		return eqcontrolgraph.Expression{Op: "constant", Value: value}
	}
	present := func(name string) *eqcontrolgraph.Expression {
		return pointer(eqcontrolgraph.Expression{Op: "present", Input: name})
	}
	less := eqcontrolgraph.Expression{Op: "less_than", Args: []eqcontrolgraph.Expression{input("gain_db"), constant(0.0)}}
	document := eqcontrolgraph.Document{
		SchemaVersion:    eqcontrolgraph.SchemaVersion,
		Plugin:           eqcontrolgraph.PluginIdentity{Name: "Fixture EQ", Manufacturer: "Fixture Maker", Format: "VST3", Version: "1.0"},
		SurfaceSignature: "p_fixture", PublicClassification: "fixed_slot_adjustable", ChannelContract: "shared",
		Bindings: map[string]eqcontrolgraph.Binding{
			"frequency": chatGraphNumber("freq", "Frequency", "Hz", 100, 10000, [][2]float64{{0, 100}, {.5, 1000}, {1, 10000}}, "logarithmic"),
			"gain":      chatGraphNumber("gain", "Gain", "dB", 0, 12, [][2]float64{{0, 0}, {.5, 6}, {1, 12}}, "linear"),
			"mode":      chatGraphEnum("mode", "Mode", map[string]eqcontrolgraph.EnumValue{"boost": {Normalized: 0, Label: "Boost"}, "cut": {Normalized: 1, Label: "Cut"}}),
			"shape":     chatGraphEnum("shape", "Shape", map[string]eqcontrolgraph.EnumValue{"bell": {Normalized: 0, Label: "Bell"}}),
			"active":    chatGraphEnum("active", "Active", map[string]eqcontrolgraph.EnumValue{"out": {Normalized: 0, Label: "Out"}, "in": {Normalized: 1, Label: "In"}}),
		},
	}
	polarity := eqcontrolgraph.Expression{Op: "select", Condition: &less,
		Then: pointer(constant("cut")), Else: pointer(constant("boost"))}
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
	return document
}

func chatControlGraphSummary(runtime *plugingrabber.EQControlGraphRuntime) map[string]any {
	return map[string]any{
		"mapping_source": "vps_control_graph", "eq_model": "fixed_slot_adjustable", "supported_filter_kinds": []string{"bell"},
		"sections": []map[string]any{{
			"section": "top", "complete": true, "addressing": "resident", "channel_bindings": []string{"shared"},
			"activation":            map[string]any{"strategy": "explicit", "known": false, "active": false},
			"control_graph_runtime": runtime,
			"shape_capabilities": []map[string]any{{"shape": "bell", "actions": map[string]any{
				"upsert": true, "modify": true, "disable": true, "remove": false,
			}}},
			"frequency_bindings": []map[string]any{{
				"param_id": "freq", "channel": "shared", "current_physical": 1000.0,
				"domain": map[string]any{"unit": "Hz", "min": 100.0, "max": 10000.0, "scale": "log"},
				"curve":  [][2]float64{{0, 100}, {.5, 1000}, {1, 10000}},
			}},
		}},
	}
}

func chatGraphNumber(id, name, unit string, min, max float64, curve [][2]float64, law string) eqcontrolgraph.Binding {
	return eqcontrolgraph.Binding{Kind: "number", ParameterID: id, ParameterName: name, Channel: "shared",
		Domain: &eqcontrolgraph.NumericDomain{Unit: unit, Minimum: min, Maximum: max, Curve: curve, ValueLaw: law}}
}

func chatGraphEnum(id, name string, values map[string]eqcontrolgraph.EnumValue) eqcontrolgraph.Binding {
	return eqcontrolgraph.Binding{Kind: "enum", ParameterID: id, ParameterName: name, Channel: "shared", Values: values}
}
