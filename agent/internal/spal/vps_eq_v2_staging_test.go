package spal

import "testing"

func TestVPSEQV2StagingAdapterReusesCompilerWithoutBecomingVerifiedProvider(t *testing.T) {
	allocation := ParameterBinding{ParameterID: "0", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"}
	definition := EQV2StagingProviderDefinition{
		Descriptor: ProviderDescriptor{ID: "forge:proq3", AdapterVersion: "v2", Status: ProviderCandidate, Experimental: true, SupportedSchemas: []string{EQPassFilterPatchControlID}},
		VPSID:      "vps-proq3", StagingID: "run-1", ImplementedSchemas: []string{EQPassFilterPatchControlID},
		Binding: EQV2Binding{HighPass: &EQV2PassFilterBinding{
			ComponentID: "b1", Allocated: &allocation,
			ResponseShape:     &EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"highpass": .25}},
			Enabled:           ParameterBinding{ParameterID: "1", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
			CutoffFrequencyHz: ParameterBinding{ParameterID: "2", Unit: "Hz", Min: 10, Max: 30000, Scale: "log"},
			SlopeDBPerOctave:  EnumParameterBinding{ParameterID: "9", Values: map[string]float64{"12": 1.0 / 9.0}},
		}},
	}
	adapter, err := NewVPSEQV2StagingAdapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Descriptor().Status != ProviderCandidate {
		t.Fatalf("staging adapter leaked a verified descriptor: %#v", adapter.Descriptor())
	}
	instruction := Instruction{SchemaID: EQPassFilterPatchControlID, TargetRef: "track:1", Parameters: map[string]float64{"enabled": 1, "cutoff_frequency_hz": 1000, "slope_db_per_octave": 12}, StringParameters: map[string]string{"filter_kind": "highpass"}}
	instance := ProviderInstance{ID: "selected", ProviderID: "forge:proq3", TargetRef: "track:1", TrackID: "1", PluginID: "2", Status: ProviderCandidate, Metadata: map[string]string{"vpsforge_staging": "true", "vps_id": "vps-proq3"}}
	binding, err := adapter.Bind(instance, instruction)
	if err != nil {
		t.Fatal(err)
	}
	writes, err := adapter.Compile(instruction, binding)
	if err != nil || len(writes) != 5 || writes[0].ParameterID != "0" || writes[1].Value != .25 {
		t.Fatalf("staging compiler writes=%#v err=%v", writes, err)
	}
	instruction.Parameters["slope_db_per_octave"] = 24
	if _, err := adapter.Bind(instance, instruction); err == nil {
		t.Fatal("unimplemented 24 dB/oct staging slope was accepted")
	}
}
