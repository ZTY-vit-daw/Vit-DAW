package spal

import "testing"

func TestVPSEQV2AdapterCompilesBandShapeAsStateOfBand(t *testing.T) {
	definition := testEQV2Definition([]string{EQBandPatchControlID})
	adapter, err := NewVPSEQV2Adapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	instruction := Instruction{SchemaID: EQBandPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"enabled": 1, "frequency_hz": 632, "gain_db": 5.4, "q": .95}, StringParameters: map[string]string{"band_ref": "b2", "response_shape": "high_shelf"}}
	instance := testEQV2Instance(definition)
	binding, err := adapter.Bind(instance, instruction)
	if err != nil {
		t.Fatal(err)
	}
	writes, err := adapter.Compile(instruction, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 5 || writes[1].ParameterID != "17" || writes[1].Value != .75 || writes[2].ParameterID != "16" {
		t.Fatalf("shape was not compiled inside Band II: %#v", writes)
	}
}

func TestVPSEQV2AdapterRejectsUnconformedShapeAndWidebandRouting(t *testing.T) {
	definition := testEQV2Definition([]string{EQBandPatchControlID, EQDynamicBandPatchControlID})
	adapter, err := NewVPSEQV2Adapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	instance := testEQV2Instance(definition)
	_, err = adapter.Bind(instance, Instruction{SchemaID: EQBandPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"frequency_hz": 500, "gain_db": 1, "q": 1}, StringParameters: map[string]string{"band_ref": "b2", "response_shape": "notch"}})
	if err == nil {
		t.Fatal("unconformed shape was accepted")
	}
	_, err = adapter.Bind(instance, Instruction{SchemaID: EQDynamicBandPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"threshold_db": -20}, StringParameters: map[string]string{"band_ref": "b2", "dynamics_mode": "normal", "routing_scope": "wideband_linked"}})
	if err == nil {
		t.Fatal("wideband-linked routing entered the generic dynamic schema")
	}
}

func TestVPSEQV2AdapterCompilesPassFilterEnumAndPartialOutput(t *testing.T) {
	definition := testEQV2Definition([]string{EQPassFilterPatchControlID, EQOutputPatchControlID})
	adapter, err := NewVPSEQV2Adapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	instance := testEQV2Instance(definition)
	pass := Instruction{SchemaID: EQPassFilterPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"enabled": 1, "cutoff_frequency_hz": 80, "slope_db_per_octave": 24}, StringParameters: map[string]string{"filter_kind": "highpass"}}
	binding, err := adapter.Bind(instance, pass)
	if err != nil {
		t.Fatal(err)
	}
	writes, err := adapter.Compile(pass, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 3 || writes[2].ParameterID != "51" || writes[2].Value < .66 || writes[2].Value > .67 {
		t.Fatalf("unexpected pass-filter writes: %#v", writes)
	}
	output := Instruction{SchemaID: EQOutputPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"output_gain_db": -2}}
	binding, err = adapter.Bind(instance, output)
	if err != nil {
		t.Fatal(err)
	}
	writes, err = adapter.Compile(output, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].ParameterID != "65" || writes[0].Value != .45 {
		t.Fatalf("unexpected output writes: %#v", writes)
	}
}

func TestVPSEQV2AdapterCompilesBandBackedPassFilterWithAllocation(t *testing.T) {
	allocation := ParameterBinding{ParameterID: "0", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"}
	definition := EQV2ProviderDefinition{
		Descriptor: ProviderDescriptor{ID: "band-backed-eq", AdapterVersion: "v2", Status: ProviderVerified, SupportedSchemas: []string{EQPassFilterPatchControlID}}, VPSID: "vps", CredentialID: "credential",
		Binding: EQV2Binding{ConformedSchemas: []string{EQPassFilterPatchControlID}, HighPass: &EQV2PassFilterBinding{
			ComponentID: "b1", Allocated: &allocation,
			ResponseShape:     &EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"highpass": .25}},
			Enabled:           ParameterBinding{ParameterID: "1", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
			CutoffFrequencyHz: ParameterBinding{ParameterID: "2", Unit: "Hz", Min: 10, Max: 30000, Scale: "log"},
			SlopeDBPerOctave:  EnumParameterBinding{ParameterID: "9", Values: map[string]float64{"12": 1.0 / 9.0}},
		}},
	}
	adapter, err := NewVPSEQV2Adapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	instruction := Instruction{SchemaID: EQPassFilterPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"enabled": 1, "cutoff_frequency_hz": 1000, "slope_db_per_octave": 12}, StringParameters: map[string]string{"filter_kind": "highpass"}}
	binding, err := adapter.Bind(testEQV2Instance(definition), instruction)
	if err != nil {
		t.Fatal(err)
	}
	writes, err := adapter.Compile(instruction, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 5 || writes[0].ParameterID != "0" || writes[0].Value != 1 || writes[1].ParameterID != "8" || writes[1].Value != .25 || writes[4].ParameterID != "9" {
		t.Fatalf("band-backed pass-filter compiler lost allocation/shape order: %#v", writes)
	}
}

func testEQV2Definition(schemas []string) EQV2ProviderDefinition {
	linear := func(id, unit string, min, max float64) ParameterBinding {
		return ParameterBinding{ParameterID: id, Unit: unit, Min: min, Max: max, Scale: "linear"}
	}
	log := func(id, unit string, min, max float64) ParameterBinding {
		return ParameterBinding{ParameterID: id, Unit: unit, Min: min, Max: max, Scale: "log"}
	}
	ratio, attack, release := linear("20", ":1", .5, 3.4), log("22", "ms", .1, 500), log("23", "ms", 10, 3000)
	bypass, dry, out := linear("62", "toggle", 0, 1), linear("64", "%", 0, 100), linear("65", "dB", -20, 20)
	return EQV2ProviderDefinition{
		Descriptor: ProviderDescriptor{ID: "eq-provider", AdapterVersion: "v2", Status: ProviderVerified, SupportedSchemas: schemas}, VPSID: "vps", CredentialID: "credential",
		Binding: EQV2Binding{ConformedSchemas: schemas, Bands: map[string]EQV2BandBinding{"b2": {ComponentID: "b2", Enabled: linear("13", "toggle", 0, 1), ResponseShape: EnumParameterBinding{ParameterID: "17", Values: map[string]float64{"low_shelf": 0, "bell": .5, "high_shelf": .75}}, FrequencyHz: log("16", "Hz", 10, 40000), GainDB: linear("14", "dB", -18, 18), Q: linear("15", "Q", .1, 6), Dynamic: &EQV2DynamicBinding{Mode: EnumParameterBinding{ParameterID: "18", Values: map[string]float64{"off": 0, "normal": .5}}, Routing: EnumParameterBinding{ParameterID: "21", Values: map[string]float64{"independent": 1}}, ThresholdDB: linear("19", "dB", -50, 0), Ratio: &ratio, AttackMS: &attack, ReleaseMS: &release}}}, HighPass: &EQV2PassFilterBinding{ComponentID: "hp", Enabled: linear("49", "toggle", 0, 1), CutoffFrequencyHz: log("50", "Hz", 10, 40000), SlopeDBPerOctave: EnumParameterBinding{ParameterID: "51", Values: map[string]float64{"6": 0, "12": 1.0 / 3, "24": 2.0 / 3, "48": 1}}}, Output: &EQV2OutputBinding{ComponentID: "io", Bypass: &bypass, DryMix: &dry, OutputGainDB: &out}},
	}
}

func testEQV2Instance(definition EQV2ProviderDefinition) ProviderInstance {
	return ProviderInstance{ID: "instance", ProviderID: definition.Descriptor.ID, TargetRef: "track:1", TrackID: "1", PluginID: "2", Status: InstanceVerified, Metadata: map[string]string{"vps_id": definition.VPSID, "vps_credential_id": definition.CredentialID, "eq_v2_ready": "true"}}
}
