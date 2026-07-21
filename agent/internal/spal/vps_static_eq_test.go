package spal

import (
	"math"
	"testing"
)

func TestVPSStaticEQAdapterCompilesOnlyCredentialMappedParameters(t *testing.T) {
	adapter, err := NewVPSStaticEQAdapter(StaticEQProviderDefinition{
		Descriptor: ProviderDescriptor{
			ID: "vps.static_eq:credential-1", AdapterVersion: "v3", Status: ProviderVerified,
			SupportedSchemas: []string{StaticBellControlID},
		},
		VPSID: "vps-1", CredentialID: "credential-1", ComponentID: "band_1", FilterTypeParameterID: "type",
		ParameterBindings: map[string]ParameterBinding{
			"enabled":             {ParameterID: "enabled", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
			"center_frequency_hz": {ParameterID: "frequency", Unit: "Hz", Min: 10, Max: 40000, Scale: "log"},
			"gain_db":             {ParameterID: "gain", Unit: "dB", Min: -18, Max: 18, Scale: "linear"},
			"q":                   {ParameterID: "q", Unit: "Q", Min: .1, Max: 6, Scale: "log"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	maxGain := 6.0
	instruction := Instruction{
		SchemaID: StaticBellControlID, TargetRef: "track:bass",
		Parameters:   map[string]float64{"center_frequency_hz": 100, "gain_db": -2.5, "q": 1.2},
		SafetyBounds: SafetyBounds{MaxAbsoluteGainDB: &maxGain},
	}
	instance := ProviderInstance{
		ID: "instance-1", ProviderID: "vps.static_eq:credential-1", TargetRef: "track:bass", TrackID: "bass", PluginID: "nova", Status: InstanceVerified,
		Metadata: map[string]string{"vps_id": "vps-1", "vps_credential_id": "credential-1", "static_bell_ready": "true"},
	}
	binding, err := adapter.Bind(instance, instruction)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := adapter.Compile(instruction, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(physical) != 4 {
		t.Fatalf("physical parameter count = %d, want 4", len(physical))
	}
	got := map[string]float64{}
	for _, parameter := range physical {
		got[parameter.ParameterID] = parameter.Value
	}
	if got["enabled"] != 1 {
		t.Fatalf("enabled = %v, want 1", got["enabled"])
	}
	wantGain := (-2.5 + 18) / 36
	if math.Abs(got["gain"]-wantGain) > 1e-9 {
		t.Fatalf("gain normalized = %.9f, want %.9f", got["gain"], wantGain)
	}
	wantFrequency := math.Log(100.0/10.0) / math.Log(40000.0/10.0)
	if math.Abs(got["frequency"]-wantFrequency) > 1e-9 {
		t.Fatalf("frequency normalized = %.9f, want %.9f", got["frequency"], wantFrequency)
	}
	if got["q"] <= 0 || got["q"] >= 1 {
		t.Fatalf("q normalized = %.9f, want value in (0, 1)", got["q"])
	}
}

func TestVPSStaticEQAdapterRejectsUnconfirmedScale(t *testing.T) {
	_, err := NewVPSStaticEQAdapter(StaticEQProviderDefinition{
		Descriptor: ProviderDescriptor{ID: "provider", AdapterVersion: "v3", Status: ProviderVerified, SupportedSchemas: []string{StaticBellControlID}},
		VPSID:      "vps", CredentialID: "credential", ComponentID: "band", FilterTypeParameterID: "type",
		ParameterBindings: map[string]ParameterBinding{
			"enabled":             {ParameterID: "enabled", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"},
			"center_frequency_hz": {ParameterID: "frequency", Unit: "Hz", Min: 10, Max: 40000, Scale: ""},
			"gain_db":             {ParameterID: "gain", Unit: "dB", Min: -18, Max: 18, Scale: "linear"},
			"q":                   {ParameterID: "q", Unit: "Q", Min: .1, Max: 6, Scale: "log"},
		},
	})
	if err == nil {
		t.Fatal("missing display scale unexpectedly created a dispatchable adapter")
	}
}
