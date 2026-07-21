package chat

import (
	"fmt"
	"math"
	"strings"
	"testing"

	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestVPSStaticEQDefinitionUsesPersistedCredentialBinding(t *testing.T) {
	minFrequency, maxFrequency := 10.0, 40000.0
	minGain, maxGain := -18.0, 18.0
	minQ, maxQ := .1, 6.0
	minEnabled, maxEnabled := 0.0, 1.0
	document := vps.VPSDocument{
		ID: "vps-1", Revision: 1,
		PluginIdentity: vps.PluginIdentity{Name: "TDR Nova", Format: "VST3", Fingerprint: vps.PluginFingerprint{ParameterSurface: "surface"}},
		Topology:       vps.Topology{Resources: []vps.PhysicalResource{{ID: "slot:band_1", Kind: "eq_band", ComponentID: "band_1", Capacity: 1}}},
		ControlSurface: vps.ControlSurface{Mappings: []vps.ControlSurfaceMapping{
			{ComponentID: "band_1", ParameterID: "type", Confirmed: true},
			{ComponentID: "band_1", ParameterID: "frequency", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "Hz", Min: &minFrequency, Max: &maxFrequency, Scale: "log"}},
			{ComponentID: "band_1", ParameterID: "gain", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "dB", Min: &minGain, Max: &maxGain, Scale: "linear"}},
			{ComponentID: "band_1", ParameterID: "q", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "Q", Min: &minQ, Max: &maxQ, Scale: "log"}},
			{ComponentID: "band_1", ParameterID: "enabled", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "toggle", Min: &minEnabled, Max: &maxEnabled, Scale: "linear"}},
		}},
	}
	credential := vps.ProviderCredential{ID: "credential-1", CapabilityID: vps.StaticEQCapabilityID, Conformance: vps.CredentialConformance{
		StaticEQBinding: &vps.StaticEQBinding{ComponentID: "band_1", FilterTypeParameterID: "type", FrequencyParameterID: "frequency", GainParameterID: "gain", QParameterID: "q", EnabledParameterID: "enabled"},
	}}
	definition, err := vpsStaticEQDefinition(document, credential)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Descriptor.ID != vpsStaticEQProviderID("credential-1") || definition.ParameterBindings["gain_db"].ParameterID != "gain" || definition.ParameterBindings["q"].Scale != "log" {
		t.Fatalf("definition did not preserve the credential binding: %#v", definition)
	}
	adapter, err := spal.NewVPSStaticEQAdapter(definition)
	if err != nil {
		t.Fatalf("definition was not dispatchable through the VPS adapter: %v", err)
	}
	if adapter.Descriptor().Experimental {
		t.Fatal("VPS Credential adapter must not be marked as the old experimental TDR adapter")
	}
}

func TestVPSStaticEQDefinitionFailsClosedWithoutPersistedBinding(t *testing.T) {
	_, err := vpsStaticEQDefinition(vps.VPSDocument{ID: "vps"}, vps.ProviderCredential{ID: "credential"})
	if err == nil || !strings.Contains(err.Error(), "rerun Plugin Learning") {
		t.Fatalf("missing persisted binding error = %v", err)
	}
}

func TestVPSStaticEQDefinitionUsesObservedQScaleWithoutChangingCredential(t *testing.T) {
	minFrequency, maxFrequency := 10.0, 40000.0
	minGain, maxGain := -18.0, 18.0
	minQ, maxQ := .1, 6.0
	minEnabled, maxEnabled := 0.0, 1.0
	document := vps.VPSDocument{
		ID: "vps-1", Revision: 1,
		PluginIdentity: vps.PluginIdentity{Name: "TDR Nova", Format: "VST3", Fingerprint: vps.PluginFingerprint{ParameterSurface: "surface"}},
		Topology:       vps.Topology{Resources: []vps.PhysicalResource{{ID: "slot:band_1", Kind: "eq_band", ComponentID: "band_1", Capacity: 1}}},
		ControlSurface: vps.ControlSurface{Mappings: []vps.ControlSurfaceMapping{
			{ComponentID: "band_1", ParameterID: "type", Confirmed: true},
			{ComponentID: "band_1", ParameterID: "frequency", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "Hz", Min: &minFrequency, Max: &maxFrequency, Scale: "log"}},
			{ComponentID: "band_1", ParameterID: "gain", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "dB", Min: &minGain, Max: &maxGain, Scale: "linear"}},
			// This reproduces the already-issued Credential: the persisted Q
			// mapping says linear even though the fresh host display probe says log.
			{ComponentID: "band_1", ParameterID: "q", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "Q", Min: &minQ, Max: &maxQ, Scale: "linear"}},
			{ComponentID: "band_1", ParameterID: "enabled", Confirmed: true, DisplayDomain: vps.DisplayDomain{Unit: "toggle", Min: &minEnabled, Max: &maxEnabled, Scale: "linear"}},
		}},
	}
	credential := vps.ProviderCredential{ID: "credential-1", CapabilityID: vps.StaticEQCapabilityID, Conformance: vps.CredentialConformance{
		StaticEQBinding: &vps.StaticEQBinding{ComponentID: "band_1", FilterTypeParameterID: "type", FrequencyParameterID: "frequency", GainParameterID: "gain", QParameterID: "q", EnabledParameterID: "enabled"},
	}}
	observedMin, observedMax := .1, 6.0
	digest := pluginParameterDigest{Parameters: []pluginParameterInfo{{
		ID: "q", HostControllable: true,
		DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{Min: &observedMin, Max: &observedMax, Scale: "log"},
	}}}
	definition, err := vpsStaticEQDefinitionWithObservedDisplaySurface(document, credential, digest)
	if err != nil {
		t.Fatal(err)
	}
	if got := definition.ParameterBindings["q"].Scale; got != "log" {
		t.Fatalf("runtime Q scale = %q, want observed log scale", got)
	}
	if got := document.ControlSurface.Mappings[3].DisplayDomain.Scale; got != "linear" {
		t.Fatalf("runtime calibration persisted a new Credential mapping: %q", got)
	}
	adapter, err := spal.NewVPSStaticEQAdapter(definition)
	if err != nil {
		t.Fatal(err)
	}
	maxSafeGain := 6.0
	instruction := spal.Instruction{
		SchemaID: spal.StaticBellControlID, TargetRef: "track:bass",
		Parameters:   map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2},
		SafetyBounds: spal.SafetyBounds{MaxAbsoluteGainDB: &maxSafeGain},
	}
	instance := spal.ProviderInstance{
		ID: "instance-bass", ProviderID: definition.Descriptor.ID, TargetRef: "track:bass", TrackID: "bass", PluginID: "nova", Status: spal.InstanceVerified,
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
	var qNormalized float64
	for _, parameter := range physical {
		if parameter.ParameterID == "q" {
			qNormalized = parameter.Value
		}
	}
	want := math.Log(1.2/.1) / math.Log(6/.1)
	if math.Abs(qNormalized-want) > 1e-9 {
		t.Fatalf("Q normalized = %.9f, want logarithmic %.9f", qNormalized, want)
	}
}

func TestVPSStaticEQCatalogEntriesExcludeNonStaticCredentials(t *testing.T) {
	entries := vpsStaticEQCatalogEntries(vps.ProviderCatalog{Entries: []vps.ProviderCatalogEntry{
		{CredentialID: "good", CapabilityID: vps.StaticEQCapabilityID, Operations: []string{vps.OperationBellCut}},
		{CredentialID: "missing-operation", CapabilityID: vps.StaticEQCapabilityID, Operations: []string{vps.OperationPatchBand}},
		{CredentialID: "other", CapabilityID: "other.v0", Operations: []string{vps.OperationBellCut}},
	}}, "")
	if len(entries) != 1 || entries[0].CredentialID != "good" {
		t.Fatalf("Catalog filter = %#v, want only verified static bell Credential", entries)
	}
}

func TestVPSStaticEQLiveSurfaceTreatsShapeAsRuntimePrecondition(t *testing.T) {
	binding := &vps.StaticEQBinding{
		ComponentID: "band_1", FilterTypeParameterID: "type", FrequencyParameterID: "frequency",
		GainParameterID: "gain", QParameterID: "q", EnabledParameterID: "enabled",
	}
	digest := pluginParameterDigest{Parameters: []pluginParameterInfo{
		{ID: "type", HostControllable: true, ValueText: "Bell"},
		{ID: "frequency", HostControllable: true},
		{ID: "gain", HostControllable: true},
		{ID: "q", HostControllable: true},
		{ID: "enabled", HostControllable: true},
	}}
	if err := validateVPSStaticEQLiveSurface(digest, binding); err != nil {
		t.Fatalf("Bell should satisfy the conformed runtime precondition: %v", err)
	}
	digest.Parameters[0].ValueText = "Low S"
	if err := validateVPSStaticEQLiveSurface(digest, binding); err == nil || !strings.Contains(err.Error(), "not Bell") {
		t.Fatalf("mutable Low S state should be a recoverable runtime precondition, got %v", err)
	}
}

func TestVPSStaticEQNotBellResponseDoesNotRequireRelearning(t *testing.T) {
	response := vpsSPALProviderResponse("conversation", agentruntime.Goal{}, "track:1", fmt.Errorf("current conformed filter type is not Bell"), nil)
	if got := cleanContextText(response.WorkflowData["canary_stage"]); got != "vps_static_bell_not_ready" {
		t.Fatalf("canary stage = %q", got)
	}
	if boolValue(response.WorkflowData["plugin_learning_required"]) {
		t.Fatalf("Bell runtime-state mismatch incorrectly required Plugin Learning: %#v", response.WorkflowData)
	}
	if strings.Contains(response.Reply, "重新运行 Plugin Learning") || !strings.Contains(response.Reply, "Credential 仍保留") {
		t.Fatalf("runtime-state guidance = %q", response.Reply)
	}
}
